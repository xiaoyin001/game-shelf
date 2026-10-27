// 文件说明：真实四服务完成三条寻宝并验证MongoDB、重登、新世界恢复与原子版本冲突。
package integration

import (
	"context"
	"errors"
	storage "game-shelf/internal/fieldbook"
	"game-shelf/internal/world"
	"game-shelf/pkg/contracts"
	"game-shelf/pkg/fieldbook"
	"github.com/coder/websocket"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"math"
	"sync"
	"testing"
	"time"
)

// bookLogin 使用注册已返回的token，避免无意义的再次认证占用真实限频窗口。
func bookLogin(t *testing.T, account string, register bool) contracts.LoginHTTPReply {
	path, status := "/login", 200
	if register {
		path, status = "/register", 201
	}
	result := accountRequest(t, path, account, testPassword, "integration")
	if result.status != status {
		t.Fatalf("book authentication: %d %s", result.status, result.body)
	}
	return result.reply
}
func bookMongo(t *testing.T) *mongo.Database {
	t.Helper()
	client, err := mongo.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:27017"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = client.Disconnect(ctx)
	})
	return client.Database("game_shelf")
}
func waitBook(t *testing.T, conn *websocket.Conn, predicate func(*contracts.FieldBook) bool) *contracts.FieldBook {
	t.Helper()
	for range 50 {
		e := read(t, conn, "state")
		if e.Book != nil && predicate(e.Book) {
			return e.Book
		}
	}
	t.Fatal("book state did not arrive")
	return nil
}
func TestLiveFieldBookStoriesPersistenceAndPrivacy(t *testing.T) {
	live(t)
	account := name()
	identity := bookLogin(t, account, true)
	a := auth(t, identity)
	peer := auth(t, bookLogin(t, name(), true))
	send(t, a, "explore", nil)
	if read(t, a, "error").Error != "no_active_trail" {
		t.Fatal("unearned discovery")
	}
	for index, entry := range fieldbook.Entries() {
		walkNPC(t, a, identity.UserID, 560, 448)
		send(t, a, "choose", map[string]any{"npc_id": "keeper", "choice_id": "trail"})
		started := read(t, a, "dialog")
		if started.Book == nil || started.Book.Trail.ID != entry.ID || started.Book.Trail.Step != 1 {
			t.Fatal(started)
		}
		for step, target := range entry.Stops {
			if target.Z == 50 {
				walkNPC(t, a, identity.UserID, 345, 300)
				send(t, a, "jump", nil)
			}
			walkNPC(t, a, identity.UserID, target.X, target.Y)
			for range 30 {
				p := findPlayer(t, read(t, a, "state").State, identity.UserID)
				if p.Grounded && math.Abs(p.Z-target.Z) <= 10 {
					break
				}
			}
			send(t, a, "explore", nil)
			if step == 0 {
				waitBook(t, a, func(b *contracts.FieldBook) bool { return b.Trail != nil && b.Trail.Step == 2 })
				// 真正换连接并验证密码；旧连接退出不能删掉新会话或清空线索。
				if index == 0 {
					a = auth(t, bookLogin(t, account, false))
					restored := read(t, a, "state")
					if restored.Book.Trail == nil || restored.Book.Trail.Step != 2 {
						t.Fatal("partial trail lost", restored)
					}
					walkNPC(t, a, identity.UserID, 560, 460)
				}
			} else {
				waitBook(t, a, func(b *contracts.FieldBook) bool { return b.Collections[index].Found && b.Trail == nil })
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, err := storage.NewStore(bookMongo(t)).Load(ctx, identity.UserID)
	if err != nil || len(saved.Found) != 3 || saved.Revision != 9 {
		t.Fatal("not durable", saved, err)
	}
	rebuilt := world.NewPersistent(storage.NewStore(bookMongo(t)))
	restored := rebuilt.Apply(&contracts.WorldReq{UserID: identity.UserID, SessionID: "rebuilt", Name: account, Action: "enter"}, time.Now())
	if restored.Error != "" || !restored.Book.Collections[2].Found {
		t.Fatal("new world did not restore", restored)
	}
	for range 5 {
		e := read(t, peer, "state")
		if e.Book == nil || e.Book.Trail != nil || e.Book.Collections[0].Found || e.Dialog != nil {
			t.Fatal("private exploration leaked", e)
		}
	}
	fresh := auth(t, bookLogin(t, account, false))
	final := read(t, fresh, "state")
	if !final.Book.Collections[2].Found || findPlayer(t, final.State, identity.UserID).Score != 0 {
		t.Fatal("collection reset or mixed with score", final)
	}
	verifyLiveCamp(t, fresh, peer, identity.UserID, account)
}
func TestLiveFieldBookMongoCASAndCorruption(t *testing.T) {
	live(t)
	db := bookMongo(t)
	store := storage.NewStore(db)
	id := bookLogin(t, name(), true).UserID
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	results := make(chan error, 12)
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Save(ctx, id, &contracts.Journal{Active: "bell"})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, storage.ErrConflict) {
			t.Fatal(err)
		}
	}
	saved, err := store.Load(ctx, id)
	if err != nil || success != 1 || saved.Revision != 1 || saved.Active != "bell" {
		t.Fatal("CAS was not unique", success, saved, err)
	}
	broken := bookLogin(t, name(), true).UserID
	_, err = db.Collection("field_books").InsertOne(ctx, bson.M{"_id": broken, "schema": 99, "revision": 0})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Load(ctx, broken); err == nil {
		t.Fatal("unknown schema reset to empty")
	}
	if _, err = store.Save(ctx, broken, &contracts.Journal{Active: "bell"}); err == nil {
		t.Fatal("newer schema overwritten")
	}
	for _, bad := range []*contracts.Journal{{Found: []string{"unknown"}}, {Active: "bell", Step: 99}, nil} {
		if _, err = store.Save(ctx, id, bad); err == nil {
			t.Fatal("invalid document accepted")
		}
	}
}
func TestLiveFieldBookRejectsForgedState(t *testing.T) {
	live(t)
	for _, field := range []string{"book", "found", "active", "step", "revision", "journal", "owner_id", "slots", "unlocked"} {
		conn := auth(t, bookLogin(t, name(), true))
		send(t, conn, "explore", map[string]any{field: 999})
		closed(t, conn)
	}
}
