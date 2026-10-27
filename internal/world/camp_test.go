// 文件说明：验证个人展台权限、距离、移动/收起、持久失败与公共快照隔离。
package world

import (
	"game-shelf/pkg/contracts"
	"sync"
	"testing"
	"time"
)

func decorateCommand(w *World, id, session, slot, item string, now time.Time) *contracts.WorldResp {
	return w.Apply(&contracts.WorldReq{UserID: id, SessionID: session, Action: "decorate", SlotID: slot, DecorationID: item}, now)
}
func TestCampOwnershipMovementAndRestore(t *testing.T) {
	now := time.Unix(1000, 0)
	store := newMemoryBooks()
	store.data["a"] = &contracts.Journal{Revision: 9, Found: []string{"bell", "letter", "flute"}}
	w := NewPersistent(store)
	enter(w, "a", "a", now)
	enter(w, "b", "b", now)
	if r := decorateCommand(w, "a", "a", "left", "bell-lantern", now); r.Error != "camp_out_of_range" {
		t.Fatal(r)
	}
	bookPosition(w, "a", 730, 500, 0)
	bookPosition(w, "b", 680, 550, 0)
	if r := decorateCommand(w, "a", "a", "unknown", "bell-lantern", now); r.Error != "invalid_camp_slot" {
		t.Fatal(r)
	}
	if r := decorateCommand(w, "b", "b", "left", "bell-lantern", now); r.Error != "decoration_locked" {
		t.Fatal("peer inherited unlock", r)
	}
	w.players["a"].state.Grounded = false
	if r := decorateCommand(w, "a", "a", "left", "bell-lantern", now); r.Error != "camp_out_of_range" {
		t.Fatal("airborne edit", r)
	}
	w.players["a"].state.Grounded = true
	actions := [][2]string{{"left", "bell-lantern"}, {"center", "bell-lantern"}, {"right", "lake-vase"}, {"center", "wind-chime"}, {"right", ""}, {"right", ""}}
	for i, action := range actions {
		r := decorateCommand(w, "a", "a", action[0], action[1], now)
		if r.Error != "" {
			t.Fatal(action, r)
		}
		if i == 1 && (len(r.Book.Camp) != 1 || r.Book.Camp[0].SlotID != "center") {
			t.Fatal("move duplicated item", r.Book)
		}
		if i == 3 && !r.Book.Decorations[0].Unlocked {
			t.Fatal("replacement consumed unlock")
		}
	}
	current := command(w, "a", "a", "poll", now)
	if len(current.Book.Camp) != 1 || current.Book.Camp[0].DecorationID != "wind-chime" || current.State.Players[0].Score != 0 {
		t.Fatal(current)
	}
	store.saveErr = true
	if r := decorateCommand(w, "a", "a", "left", "bell-lantern", now); r.Error != "progress_unavailable" || len(w.players["a"].journal.Camp) != 1 {
		t.Fatal("failed write became visible", r)
	}
	store.saveErr = false
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() { defer wg.Done(); decorateCommand(w, "a", "a", "left", "bell-lantern", now) }()
	}
	wg.Wait()
	peer := command(w, "b", "b", "poll", now)
	if len(peer.Book.Camp) != 0 || peer.Book.Collections[0].Found {
		t.Fatal("private book leaked")
	}
	var display *contracts.CampDisplay
	for _, camp := range peer.State.Camps {
		if camp.OwnerID == "a" {
			display = camp
		}
	}
	if display == nil || len(display.Slots) != 2 {
		t.Fatal("peer missing public display", display)
	}
	display.Slots[0].DecorationID = "mutated"
	if command(w, "a", "a", "poll", now).Book.Camp[0].DecorationID == "mutated" {
		t.Fatal("public response changed saved state")
	}
	command(w, "a", "a", "leave", now)
	rebuilt := NewPersistent(store)
	restored := enter(rebuilt, "a", "new", now)
	if len(restored.Book.Camp) != 2 || !restored.Book.Collections[2].Found {
		t.Fatal("camp not restored", restored)
	}
}
