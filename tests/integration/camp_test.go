// 文件说明：复用真实寻宝所得收藏验证营地布置、公共参观、重登恢复及旧存档升级。
package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	storage "game-shelf/internal/fieldbook"
	"game-shelf/internal/world"
	"game-shelf/pkg/contracts"
	"game-shelf/pkg/fieldbook"
	"github.com/coder/websocket"
	"go.mongodb.org/mongo-driver/v2/bson"
	"testing"
	"time"
)

func verifyLiveCamp(t *testing.T, actor, peer *websocket.Conn, ownerID, account string) {
	t.Helper()
	decorate := func(conn *websocket.Conn, slot, item string) {
		send(t, conn, "decorate", map[string]any{"slot_id": slot, "decoration_id": item})
	}
	decorate(actor, "left", "bell-lantern")
	if read(t, actor, "error").Error != "camp_out_of_range" {
		t.Fatal("remote camp edit")
	}
	decorate(peer, "left", "bell-lantern")
	if read(t, peer, "error").Error != "decoration_locked" {
		t.Fatal("visitor inherited unlock")
	}
	walkNPC(t, actor, ownerID, 560, 460)
	walkNPC(t, actor, ownerID, 730, 500)
	decorate(actor, "fake", "bell-lantern")
	if read(t, actor, "error").Error != "invalid_camp_slot" {
		t.Fatal("unknown slot accepted")
	}
	actions := [][2]string{{"left", "bell-lantern"}, {"center", "bell-lantern"}, {"right", "lake-vase"}, {"center", ""}, {"center", "bell-lantern"}}
	for _, action := range actions {
		decorate(actor, action[0], action[1])
		slot, item := action[0], action[1]
		waitBook(t, actor, func(book *contracts.FieldBook) bool {
			for _, placed := range book.Camp {
				if placed.SlotID == slot {
					return placed.DecorationID == item
				}
			}
			return item == ""
		})
	}
	observed := false
	for range 100 {
		event := read(t, peer, "state")
		if event.Book.Collections[0].Found || len(event.Book.Camp) != 0 {
			t.Fatal("camp visit leaked private progress")
		}
		for _, camp := range event.State.Camps {
			if camp.OwnerID == ownerID && len(camp.Slots) == 2 && camp.Slots[0].SlotID == "center" && camp.Slots[0].DecorationID == "bell-lantern" && camp.Slots[1].DecorationID == "lake-vase" {
				observed = true
			}
		}
		if observed {
			break
		}
	}
	if !observed {
		t.Fatal("peer did not see authoritative public camp")
	}
	next := auth(t, bookLogin(t, account, false))
	restored := read(t, next, "state")
	if len(restored.Book.Camp) != 2 || !restored.Book.Decorations[2].Unlocked || findPlayer(t, restored.State, ownerID).Score != 0 {
		t.Fatal("camp not restored", restored)
	}
	db := bookMongo(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var raw struct {
		Schema  int               `bson:"schema"`
		Journal contracts.Journal `bson:",inline"`
	}
	if err := db.Collection("field_books").FindOne(ctx, bson.M{"_id": ownerID}).Decode(&raw); err != nil || raw.Schema != 2 || len(raw.Journal.Camp) != 2 || raw.Journal.Revision != 14 {
		t.Fatal("camp not durable", raw, err)
	}
	rebuilt := world.NewPersistent(storage.NewStore(db))
	result := rebuilt.Apply(&contracts.WorldReq{Action: "enter", UserID: ownerID, SessionID: "camp-rebuilt", Name: account}, time.Now())
	if result.Error != "" || len(result.Book.Camp) != 2 || len(result.State.Camps) != 1 || len(result.State.Camps[0].Slots) != 2 {
		t.Fatal("new world lost display", result)
	}
}
func TestLiveFieldBookLegacyCampMigration(t *testing.T) {
	live(t)
	// 此项只验真实存储迁移，使用本次随机ID，不额外消耗HTTP认证窗口。
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		t.Fatal(err)
	}
	id := hex.EncodeToString(bytes)
	db := bookMongo(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := db.Collection("field_books").InsertOne(ctx, bson.M{"_id": id, "schema": 1, "revision": 3, "found": []string{"bell"}, "active": "", "step": 0}); err != nil {
		t.Fatal(err)
	}
	store := storage.NewStore(db)
	legacy, err := store.Load(ctx, id)
	if err != nil || legacy.Revision != 3 || len(legacy.Camp) != 0 || len(legacy.Found) != 1 {
		t.Fatal("legacy book not readable", legacy, err)
	}
	legacy.Camp = []*contracts.CampSlot{{SlotID: "left", DecorationID: "bell-lantern"}}
	updated, err := store.Save(ctx, id, legacy)
	if err != nil || updated.Revision != 4 || len(updated.Found) != 1 {
		t.Fatal("migration lost progress", updated, err)
	}
	var raw bson.M
	if err := db.Collection("field_books").FindOne(ctx, bson.M{"_id": id}).Decode(&raw); err != nil || raw["schema"] != int32(2) {
		t.Fatal("schema not upgraded", raw, err)
	}
	for _, bad := range []*contracts.Journal{
		{Revision: 4, Found: []string{"bell"}, Camp: []*contracts.CampSlot{{SlotID: "right", DecorationID: "lake-vase"}}},
		{Revision: 4, Found: []string{"bell"}, Camp: []*contracts.CampSlot{{SlotID: "fake", DecorationID: "bell-lantern"}}},
		{Revision: fieldbook.MaxRevision, Found: []string{"bell"}},
	} {
		if _, err := store.Save(ctx, id, bad); err == nil {
			t.Fatal("unsafe camp or exhausted version accepted", bad)
		}
	}
	recovered, err := store.Load(ctx, id)
	if err != nil || recovered.Revision != 4 || recovered.Camp[0].DecorationID != "bell-lantern" {
		t.Fatal("failed write corrupted book", recovered, err)
	}
}
