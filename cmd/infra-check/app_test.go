// 文件说明：验证假就绪不能放行，重试可恢复且取消/超时有真实错误。
package main

import (
	"context"
	"errors"
	"game-shelf/internal/config"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
func TestWaitReadyRequiresSuccessfulProtocolCheck(t *testing.T) {
	attempts := 0
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := waitReady(ctx, time.Millisecond, func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("not ready")
		}
		return nil
	}, quiet())
	if err != nil || attempts != 3 {
		t.Fatal("did not wait for successful protocol check", err, attempts)
	}
}
func TestWaitReadyPreservesTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := waitReady(ctx, time.Millisecond, func(context.Context) error { return errors.New("mongo unavailable") }, quiet())
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "mongo unavailable") {
		t.Fatal("timeout evidence lost", err)
	}
}
func TestCancelledWaitDoesNotProbe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	err := waitReady(ctx, time.Millisecond, func(context.Context) error { called = true; return nil }, quiet())
	if !errors.Is(err, context.Canceled) || called {
		t.Fatal("cancelled wait performed another probe", err, called)
	}
}

func TestOpenTCPPortIsNotMongoReadiness(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func(conn net.Conn) {
				defer wg.Done()
				defer conn.Close()
				stop := context.AfterFunc(ctx, func() { conn.Close() })
				defer stop()
				io.Copy(io.Discard, conn)
			}(conn)
		}
	}()
	defer func() { cancel(); listener.Close(); wg.Wait() }()
	app := config.App{Mongo: config.Mongo{URI: "mongodb://" + listener.Addr().String()}}
	err = checkDependencies(ctx, app)
	if err == nil || !strings.Contains(err.Error(), "mongo") {
		t.Fatal("TCP-only listener passed as MongoDB", err)
	}
}
