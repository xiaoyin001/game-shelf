// 文件说明：负责初始化 gate 的公网 TCP、内部 TCP、服务发现、Redis 和健康检查。
package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"game-shelf/internal/config"
	"game-shelf/internal/discovery"
	"game-shelf/internal/health"
	"game-shelf/internal/logx"
	"game-shelf/internal/redisx"
	appruntime "game-shelf/internal/runtime"
	"game-shelf/internal/transport"

	"github.com/redis/go-redis/v9"
)

func runGate() {
	configPath := flag.String("config", "configs/local/gate.json", "path to json config")
	flag.Parse()

	app, hash, err := config.Load(*configPath)
	if err != nil {
		panic(err)
	}
	log, closer := logx.MustNew(app.Log)
	if closer != nil {
		defer func() { _ = closer.Close() }()
	}
	log = log.With("service", app.Service.Name, "instance_id", app.Service.InstanceID, "version", app.Service.Version)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	healthServer := health.New(app.Health.ListenAddr, app.Service.Name, app.Service.InstanceID, app.Service.Version, log)
	healthServer.Start()
	defer appruntime.ShutdownHealth(healthServer)

	etcdClient, registrar := appruntime.StartDiscovery(ctx, app, healthServer, log)
	if etcdClient == nil || registrar == nil {
		log.Error("etcd unavailable after startup grace")
		os.Exit(1)
	}
	defer func() {
		if registrar != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			registrar.Stop(shutdownCtx)
		}
		if etcdClient != nil {
			_ = etcdClient.Close()
		}
	}()

	var redisClient *redis.Client
	if err := appruntime.WaitReady(ctx, app, "redis", log, func(checkCtx context.Context) error {
		var connectErr error
		redisClient, connectErr = redisx.Connect(checkCtx, app.Redis)
		return connectErr
	}); err != nil {
		log.Error("redis connect failed", "error", err)
		healthServer.SetDependency("redis", "not_ready")
		os.Exit(1)
	}
	healthServer.SetDependency("redis", "ready")
	defer func() { _ = redisClient.Close() }()

	service := NewGateService(app, log, etcdClient, redisClient)
	if app.WebSocket.ListenAddr != "" {
		listener, err := net.Listen("tcp", app.WebSocket.ListenAddr)
		if err != nil {
			log.Error("websocket listen failed", "error", err)
			return
		}
		web := &http.Server{ReadHeaderTimeout: 2 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { service.serveWebSocket(ctx, w, r) })}
		defer func() {
			end, cancel := context.WithTimeout(context.Background(), app.ShutdownTimeout())
			defer cancel()
			_ = web.Shutdown(end)
		}()
		go func() {
			if err := web.Serve(listener); err != nil && err != http.ErrServerClosed {
				log.Error("websocket stopped", "error", err)
				stop()
			}
		}()
	}
	publicServer := transport.NewServer(app.Public.ListenAddr, app.Limit.MaxBodyBytes, app.ClientIdleTimeout(), log, service.HandleClient)
	if err := publicServer.Start(ctx); err != nil {
		log.Error("gate public tcp server init failed", "error", err)
		os.Exit(1)
	}
	defer publicServer.Shutdown()

	internalServer := transport.NewServer(app.Internal.ListenAddr, app.Limit.MaxBodyBytes, app.InternalIdleTimeout(), log, service.HandleInternal)
	if err := internalServer.Start(ctx); err != nil {
		log.Error("gate internal tcp server init failed", "error", err)
		os.Exit(1)
	}
	defer internalServer.Shutdown()

	go appruntime.MonitorDependencies(ctx, app, healthServer, registrar, log, stop, func(checkCtx context.Context) map[string]error {
		deps := map[string]error{
			"etcd":  discovery.Check(checkCtx, etcdClient, app.Discovery),
			"redis": redisClient.Ping(checkCtx).Err(),
		}
		lobbies, err := discovery.ListReady(checkCtx, etcdClient, app.Discovery.BasePath, "lobby")
		if err != nil {
			deps["lobby"] = err
		} else if len(lobbies) == 0 {
			deps["lobby"] = context.DeadlineExceeded
		} else {
			deps["lobby"] = nil
		}
		return deps
	})

	go config.Watch(ctx, *configPath, app.WatchInterval(), log, func(next config.App, nextHash string) {
		log.Info("config hot reloaded", "old_hash", hash, "new_hash", nextHash, "gray", next.Service.Gray, "weight", next.Service.Weight)
		hash = nextHash
	})

	<-ctx.Done()
	healthServer.SetStatus(health.StatusDraining)
	appruntime.UpdateRegistrar(context.Background(), registrar, health.StatusDraining, 0, log)
	healthServer.SetStatus(health.StatusStopping)
	log.Info("service stopped")
}
