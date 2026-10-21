// 文件说明：验证服务取消能关闭空闲连接，且内部调用尊重更短的 context 截止时间。
package transport

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"game-shelf/internal/protocol"
	"game-shelf/pkg/contracts"
)

func TestCancelClosesIdleConnections(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	server := NewServer("127.0.0.1:0", 4096, time.Minute, slog.New(slog.NewTextHandler(io.Discard, nil)), func(context.Context, *Conn, protocol.Message) (protocol.Message, bool) {
		return protocol.Message{}, false
	})
	if err := server.Start(ctx); err != nil {
		t.Fatal(err)
	}
	conn, err := net.Dial("tcp", server.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// 发一个完整包确保连接已经由 acceptLoop 接收。
	msg, _ := protocol.MarshalMessage(1, 1, &contracts.PingReq{})
	if err := protocol.WriteMessage(conn, msg); err != nil {
		t.Fatal(err)
	}
	cancel()
	done := make(chan struct{})
	go func() { server.Shutdown(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown blocked on idle connection")
	}
}
func TestClientAndServerExchangeProtobuf(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := NewServer("127.0.0.1:0", 4096, time.Second, slog.Default(), func(_ context.Context, _ *Conn, msg protocol.Message) (protocol.Message, bool) {
		var req contracts.PingReq
		if err := protocol.UnmarshalBody(msg, &req); err != nil {
			t.Error(err)
		}
		reply, err := protocol.MarshalMessage(2, msg.Seq, &contracts.PingResp{UnixMillis: req.UnixMillis})
		return reply, err == nil
	})
	if err := server.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); server.Shutdown() }()
	client := NewClient(server.listener.Addr().String(), time.Second, 4096, slog.Default())
	defer client.Close()
	var reply contracts.PingResp
	if err := client.Call(ctx, 1, 2, &contracts.PingReq{UnixMillis: 42}, &reply); err != nil || reply.UnixMillis != 42 {
		t.Fatal("exchange failed", err, &reply)
	}
}

func TestCallRespectsContextDeadline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			defer conn.Close()
			_, _ = io.Copy(io.Discard, conn)
		}
	}()
	client := NewClient(listener.Addr().String(), 3*time.Second, 4096, slog.Default())
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	err = client.Call(ctx, 1, 2, &contracts.PingReq{}, &contracts.PingResp{})
	if err == nil {
		t.Fatal("unresponsive peer succeeded")
	}
	if time.Since(start) > time.Second {
		t.Fatal("call ignored context deadline")
	}
}
