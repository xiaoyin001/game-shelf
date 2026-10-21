// 文件说明：负责初始化 login 的公网 HTTP、服务发现、Redis 和健康检查。
package main

import (
	"context"
	"flag"
	"log/slog"
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
	"game-shelf/internal/session"

	"github.com/redis/go-redis/v9"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func runLogin() {
	configPath := flag.String("config", "configs/local/login.json", "path to json config")
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

	handler := &LoginHTTP{
		app:   app,
		log:   log,
		redis: redisClient,
		etcd:  etcdClient,
		ctx:   ctx,
	}
	if redisClient != nil {
		handler.store = session.NewStore(redisClient)
	}

	publicServer := &http.Server{
		Addr:              app.Public.ListenAddr,
		Handler:           handler.routes(),
		ReadHeaderTimeout: 2 * time.Second,
	}
	go func() {
		log.Info("login public http server starting", "addr", app.Public.ListenAddr)
		if err := publicServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("login public http server stopped", "error", err)
			stop()
		}
	}()

	go appruntime.MonitorDependencies(ctx, app, healthServer, registrar, log, stop, func(checkCtx context.Context) map[string]error {
		deps := map[string]error{
			"etcd":  discovery.Check(checkCtx, etcdClient, app.Discovery),
			"redis": redisClient.Ping(checkCtx).Err(),
		}
		deps["db"] = readyInstances(checkCtx, etcdClient, app, "db")
		deps["gate"] = readyInstances(checkCtx, etcdClient, app, "gate")
		return deps
	})

	go config.Watch(ctx, *configPath, app.WatchInterval(), log, func(next config.App, nextHash string) {
		log.Info("config hot reloaded", "old_hash", hash, "new_hash", nextHash, "gray", next.Service.Gray, "weight", next.Service.Weight)
		hash = nextHash
	})

	<-ctx.Done()
	healthServer.SetStatus(health.StatusDraining)
	appruntime.UpdateRegistrar(context.Background(), registrar, health.StatusDraining, 0, log)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), app.ShutdownTimeout())
	defer cancel()
	_ = publicServer.Shutdown(shutdownCtx)
	healthServer.SetStatus(health.StatusStopping)
	log.Info("service stopped")
}

func readyInstances(ctx context.Context, etcdClient *clientv3.Client, app config.App, serviceName string) error {
	instances, err := discovery.ListReady(ctx, etcdClient, app.Discovery.BasePath, serviceName)
	if err != nil {
		return err
	}
	if len(instances) == 0 {
		return context.DeadlineExceeded
	}
	return nil
}
