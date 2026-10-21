// 文件说明：用业务同款驱动等待真实依赖就绪，端口接通本身不能放行。
package main

import (
	"context"
	"flag"
	"fmt"
	"game-shelf/internal/config"
	"game-shelf/internal/discovery"
	"game-shelf/internal/logx"
	"game-shelf/internal/mongox"
	"game-shelf/internal/redisx"
	"log/slog"
	"time"
)

func run() error {
	path := flag.String("config", "configs/local/development.json", "依赖检查配置路径")
	flag.Parse()
	app, _, err := config.Load(*path)
	if err != nil {
		return err
	}
	log, closer := logx.MustNew(app.Log)
	if closer != nil {
		defer closer.Close()
	}
	slog.SetDefault(log)
	ctx, cancel := context.WithTimeout(context.Background(), app.StartupGrace())
	defer cancel()
	return waitReady(ctx, app.CheckInterval(), func(parent context.Context) error {
		checkCtx, done := context.WithTimeout(parent, app.CheckInterval())
		defer done()
		return checkDependencies(checkCtx, app)
	}, log)
}
func waitReady(ctx context.Context, interval time.Duration, check func(context.Context) error, log *slog.Logger) error {
	var last error
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("依赖未就绪：%w；最近错误：%v", err, last)
		}
		last = check(ctx)
		if last == nil {
			log.Info("真实依赖协议检查通过")
			return nil
		}
		log.Warn("等待真实依赖就绪", "error", last)
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("依赖未就绪：%w；最近错误：%v", ctx.Err(), last)
		case <-timer.C:
		}
	}
}
func checkDependencies(ctx context.Context, app config.App) error {
	mongo, err := mongox.Connect(ctx, app.Mongo)
	if err != nil {
		return fmt.Errorf("mongo: %w", err)
	}
	// 释放连接使用独立的短截止时间，检查超时不能跳过驱动清理。
	cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
	err = mongo.Disconnect(cleanup)
	cancel()
	if err != nil {
		return fmt.Errorf("mongo cleanup: %w", err)
	}
	redis, err := redisx.Connect(ctx, app.Redis)
	if err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	if err := redis.Close(); err != nil {
		return err
	}
	etcd, err := discovery.NewClient(app.Discovery)
	if err != nil {
		return err
	}
	defer etcd.Close()
	if err := discovery.Check(ctx, etcd, app.Discovery); err != nil {
		return fmt.Errorf("etcd: %w", err)
	}
	return nil
}
