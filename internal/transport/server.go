// 文件说明：提供内部服务和 gate 客户端入口共用的 TCP 长连接服务端。
package transport

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"time"

	"game-shelf/internal/protocol"
)

type Handler func(context.Context, *Conn, protocol.Message) (protocol.Message, bool)

type Server struct {
	addr         string
	maxBodyBytes int
	idleTimeout  time.Duration
	log          *slog.Logger
	handler      Handler

	listener net.Listener
	wg       sync.WaitGroup
}

type Conn struct {
	conn net.Conn
	mu   sync.Mutex
}

func NewServer(addr string, maxBodyBytes int, idleTimeout time.Duration, log *slog.Logger, handler Handler) *Server {
	return &Server{
		addr:         addr,
		maxBodyBytes: maxBodyBytes,
		idleTimeout:  idleTimeout,
		log:          log,
		handler:      handler,
	}
}

func (s *Server) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.listener = listener
	s.log.Info("tcp server starting", "addr", s.addr)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		<-ctx.Done()
		_ = listener.Close()
	}()

	s.wg.Add(1)
	go s.acceptLoop(ctx, listener)
	return nil
}

func (s *Server) Shutdown() {
	if s.listener != nil {
		_ = s.listener.Close()
	}
	s.wg.Wait()
}

func (s *Server) acceptLoop(ctx context.Context, listener net.Listener) {
	defer s.wg.Done()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			s.log.Error("tcp accept failed", "error", err)
			continue
		}
		wrapped := &Conn{conn: conn}
		s.wg.Add(1)
		go s.handleConn(ctx, wrapped)
	}
}

func (s *Server) handleConn(ctx context.Context, conn *Conn) {
	defer s.wg.Done()
	defer func() { _ = conn.conn.Close() }()
	// 停服时立即打断空闲连接，不能让 Shutdown 等到心跳超时。
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	for {
		protocol.SetDeadline(conn.conn, s.idleTimeout)
		msg, err := protocol.ReadMessage(conn.conn, s.maxBodyBytes)
		if err != nil {
			return
		}
		reply, ok := s.handler(ctx, conn, msg)
		if !ok {
			continue
		}
		if err := conn.Write(reply); err != nil {
			return
		}
	}
}

func (c *Conn) Write(msg protocol.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return protocol.WriteMessage(c.conn, msg)
}

func (c *Conn) Close() error {
	return c.conn.Close()
}
