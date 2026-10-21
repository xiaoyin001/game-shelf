// 文件说明：提供内部服务间自定义 TCP 长连接请求响应客户端。
package transport

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"game-shelf/internal/protocol"
	"game-shelf/pkg/contracts"

	"github.com/golang/protobuf/proto"
)

type Client struct {
	addr         string
	timeout      time.Duration
	maxBodyBytes int
	log          *slog.Logger

	mu   sync.Mutex
	conn net.Conn
	seq  atomic.Uint32
}

func NewClient(addr string, timeout time.Duration, maxBodyBytes int, log *slog.Logger) *Client {
	return &Client{
		addr:         addr,
		timeout:      timeout,
		maxBodyBytes: maxBodyBytes,
		log:          log,
	}
}

func (c *Client) Call(ctx context.Context, reqID, respID uint32, req proto.Message, resp proto.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}
	conn, err := c.ensureConn(ctx)
	if err != nil {
		return err
	}

	seq := c.seq.Add(1)
	msg, err := protocol.MarshalMessage(reqID, seq, req)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(c.timeout)
	if end, ok := ctx.Deadline(); ok && end.Before(deadline) {
		deadline = end
	}
	_ = conn.SetDeadline(deadline)
	if err := protocol.WriteMessage(conn, msg); err != nil {
		c.closeLocked()
		return err
	}

	reply, err := protocol.ReadMessage(conn, c.maxBodyBytes)
	if err != nil {
		c.closeLocked()
		return err
	}
	if reply.Seq != seq {
		c.closeLocked()
		return fmt.Errorf("unexpected seq: got %d want %d", reply.Seq, seq)
	}
	if reply.ProtoID != respID {
		return fmt.Errorf("unexpected proto id: got %d want %d", reply.ProtoID, respID)
	}
	return protocol.UnmarshalBody(reply, resp)
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeLocked()
}

func (c *Client) StartHeartbeat(ctx context.Context, reqID, respID uint32, interval time.Duration) {
	if interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pingCtx, cancel := context.WithTimeout(ctx, c.timeout)
				var resp contracts.PingResp
				err := c.Call(pingCtx, reqID, respID, &contracts.PingReq{UnixMillis: time.Now().UnixMilli()}, &resp)
				cancel()
				if err != nil {
					c.log.Error("tcp heartbeat failed", "addr", c.addr, "error", err)
				}
			}
		}
	}()
}

func (c *Client) ensureConn(ctx context.Context) (net.Conn, error) {
	if c.conn != nil {
		return c.conn, nil
	}
	dialer := net.Dialer{Timeout: c.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return nil, err
	}
	c.log.Info("tcp client connected", "addr", c.addr)
	c.conn = conn
	return conn, nil
}

func (c *Client) closeLocked() error {
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
}
