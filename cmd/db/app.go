// 文件说明：负责初始化 db 服务依赖、注册发现、健康检查和内部 TCP 服务。
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"game-shelf/internal/config"
	"game-shelf/internal/credentials"
	"game-shelf/internal/discovery"
	"game-shelf/internal/fieldbook"
	"game-shelf/internal/health"
	"game-shelf/internal/logx"
	"game-shelf/internal/mongox"
	appruntime "game-shelf/internal/runtime"
	"game-shelf/internal/scores"
	"game-shelf/internal/transport"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

func runDB() {
	configPath := flag.String("config", "configs/local/db.json", "path to json config")
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

	var mongoClient *mongo.Client
	if err := appruntime.WaitReady(ctx, app, "mongo", log, func(checkCtx context.Context) error {
		var connectErr error
		mongoClient, connectErr = mongox.Connect(checkCtx, app.Mongo)
		return connectErr
	}); err != nil {
		log.Error("mongo connect failed", "error", err)
		healthServer.SetDependency("mongo", "not_ready")
		os.Exit(1)
	}
	healthServer.SetDependency("mongo", "ready")
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = mongoClient.Disconnect(shutdownCtx)
	}()

	accounts, err := credentials.NewStore(mongoClient.Database(app.Mongo.Database))
	if err != nil {
		log.Error("account verifier init failed", "error", err)
		return
	}
	indexCtx, indexCancel := context.WithTimeout(ctx, 5*time.Second)
	scoreStore, err := scores.NewStore(indexCtx, mongoClient.Database(app.Mongo.Database))
	indexCancel()
	if err != nil {
		log.Error("score indexes unavailable", "error", err)
		return
	}
	service := &DBService{app: app, log: log, accounts: accounts, books: fieldbook.NewStore(mongoClient.Database(app.Mongo.Database)), scores: scoreStore}
	tcpServer := transport.NewServer(app.Internal.ListenAddr, app.Limit.MaxBodyBytes, app.InternalIdleTimeout(), log, service.Handle)
	if err := tcpServer.Start(ctx); err != nil {
		log.Error("internal tcp server init failed", "error", err)
		os.Exit(1)
	}
	defer tcpServer.Shutdown()

	go appruntime.MonitorDependencies(ctx, app, healthServer, registrar, log, stop, func(checkCtx context.Context) map[string]error {
		return map[string]error{
			"etcd":  discovery.Check(checkCtx, etcdClient, app.Discovery),
			"mongo": mongoClient.Ping(checkCtx, readpref.Primary()),
		}
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
