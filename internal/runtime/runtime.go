// 文件说明：提供服务启动阶段共享的发现注册、状态更新和健康检查关闭辅助逻辑。
package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"game-shelf/internal/config"
	"game-shelf/internal/discovery"
	"game-shelf/internal/health"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func StartDiscovery(ctx context.Context, app config.App, healthServer *health.Server, log *slog.Logger) (*clientv3.Client, *discovery.Registrar) {
	deadline := time.Now().Add(app.StartupGrace())
	for {
		client, err := discovery.NewClient(app.Discovery)
		if err == nil {
			registrar := discovery.NewRegistrar(client, app, health.StatusStarting)
			if err := registrar.Start(ctx); err == nil {
				healthServer.SetDependency("etcd", "ready")
				return client, registrar
			}
			log.Error("service register failed", "error", err)
			_ = client.Close()
		} else {
			log.Error("etcd client init failed", "error", err)
		}
		healthServer.SetDependency("etcd", "not_ready")
		if time.Now().After(deadline) {
			return nil, nil
		}
		select {
		case <-ctx.Done():
			return nil, nil
		case <-time.After(app.CheckInterval()):
		}
	}
}

func UpdateRegistrar(ctx context.Context, registrar *discovery.Registrar, status string, currentConn int64, log *slog.Logger) {
	if registrar == nil {
		return
	}
	if err := registrar.Update(ctx, status, currentConn); err != nil {
		log.Error("service status update failed", "status", status, "error", err)
	}
}

func ShutdownHealth(server *health.Server) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func MonitorDependencies(ctx context.Context, app config.App, healthServer *health.Server, registrar *discovery.Registrar, log *slog.Logger, stop context.CancelFunc, check func(context.Context) map[string]error) {
	ticker := time.NewTicker(app.CheckInterval())
	defer ticker.Stop()

	var unavailableSince time.Time
	for {
		ready := updateDependencies(ctx, app, healthServer, check)
		status := health.StatusReady
		if !ready {
			status = health.StatusNotReady
		}
		healthServer.SetStatus(status)
		UpdateRegistrar(ctx, registrar, status, 0, log)

		if ready {
			unavailableSince = time.Time{}
		} else if unavailableSince.IsZero() {
			unavailableSince = time.Now()
		} else if time.Since(unavailableSince) >= app.UnavailableExitAfter() {
			log.Error("dependency unavailable for too long, stopping service", "duration", time.Since(unavailableSince).String())
			stop()
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func WaitReady(ctx context.Context, app config.App, name string, log *slog.Logger, check func(context.Context) error) error {
	deadline := time.Now().Add(app.StartupGrace())
	for {
		checkCtx, cancel := context.WithTimeout(ctx, app.CheckInterval())
		err := check(checkCtx)
		cancel()
		if err == nil {
			return nil
		}
		log.Error("dependency not ready", "dependency", name, "error", err)
		if time.Now().After(deadline) {
			return fmt.Errorf("%s unavailable after startup grace: %w", name, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(app.CheckInterval()):
		}
	}
}

func updateDependencies(ctx context.Context, app config.App, healthServer *health.Server, check func(context.Context) map[string]error) bool {
	checkCtx, cancel := context.WithTimeout(ctx, app.CheckInterval())
	defer cancel()

	deps := check(checkCtx)
	ready := true
	for name, err := range deps {
		if err != nil {
			ready = false
			healthServer.SetDependency(name, "not_ready")
			continue
		}
		healthServer.SetDependency(name, "ready")
	}
	return ready
}
