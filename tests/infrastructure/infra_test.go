// 文件说明：以真实 MongoDB、etcd 与 Redis 验证驱动、租约、单次凭据及会话替换。
package infrastructure

import (
	"context"
	"fmt"
	"game-shelf/internal/config"
	"game-shelf/internal/discovery"
	"game-shelf/internal/mongox"
	"game-shelf/internal/redisx"
	"game-shelf/internal/session"
	"os"
	"testing"
	"time"
)

func TestRealDependenciesAndAtomicSessions(t *testing.T) {
	if os.Getenv("GAME_SHELF_INFRA") != "1" {
		t.Skip("执行 npm run test:infra 才启动真实依赖")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	app, _, err := config.Load("../../configs/local/development.json")
	if err != nil {
		t.Fatal(err)
	}
	mongo, err := mongox.Connect(ctx, app.Mongo)
	if err != nil {
		t.Fatal(err)
	}
	defer mongo.Disconnect(ctx)
	etcd, err := discovery.NewClient(app.Discovery)
	if err != nil {
		t.Fatal(err)
	}
	defer etcd.Close()
	app.Discovery.BasePath = "/game-shelf/validation"
	app.Service.InstanceID = fmt.Sprint("test-", time.Now().UnixNano())
	registrar := discovery.NewRegistrar(etcd, app, "ready")
	if err := registrar.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer registrar.Stop(context.Background())
	list, err := discovery.ListReady(ctx, etcd, app.Discovery.BasePath, app.Service.Name)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, v := range list {
		if v.InstanceID == app.Service.InstanceID {
			found = true
		}
	}
	if !found {
		t.Fatal("leased registration not discoverable")
	}
	redis, err := redisx.Connect(ctx, app.Redis)
	if err != nil {
		t.Fatal(err)
	}
	defer redis.Close()
	store := session.NewStore(redis)
	token, err := store.CreateLoginToken(ctx, app.Service.InstanceID, "test", "infra", "test-gate", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	consumed, err := store.ConsumeLoginToken(ctx, token.Token)
	if err != nil || consumed.UserID != token.UserID {
		t.Fatal("consume failed", err)
	}
	if _, err := store.ConsumeLoginToken(ctx, token.Token); err == nil {
		t.Fatal("token replay")
	}
	old, _, err := store.CreateOnlineSession(ctx, token, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	fresh, previous, err := store.CreateOnlineSession(ctx, token, time.Second)
	if err != nil || previous == nil || previous.SessionID != old.SessionID {
		t.Fatal("replacement not atomic", err)
	}
	if store.RenewOnlineSession(ctx, old.SessionID, time.Second) == nil {
		t.Fatal("replaced session renewed")
	}
	if err := store.EndOnlineSession(ctx, old.SessionID, old.UserID); err != nil {
		t.Fatal(err)
	}
	if err := store.RenewOnlineSession(ctx, fresh.SessionID, time.Second); err != nil {
		t.Fatal("old cleanup removed new session", err)
	}
	_ = store.EndOnlineSession(ctx, fresh.SessionID, fresh.UserID)
	expired, err := store.CreateLoginToken(ctx, "expired-test", "test", "infra", "test-gate", 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if _, err := store.ConsumeLoginToken(ctx, expired.Token); err == nil {
		t.Fatal("expired token accepted")
	}
}
