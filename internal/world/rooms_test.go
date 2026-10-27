// 文件说明：验证双人固定席位、共享配对、20秒重连与独立Tick，不靠客户端时钟推进。
package world

import (
	"game-shelf/pkg/contracts"
	"sync"
	"testing"
	"time"
)

func roomReq(w *World, id, action, roomID string, n time.Time, ready bool) *contracts.WorldResp {
	return w.Apply(&contracts.WorldReq{UserID: id, SessionID: id, Action: action, RoomID: roomID, Ready: ready}, n)
}
func duo(t *testing.T) (*World, time.Time, *contracts.WorldResp) {
	t.Helper()
	n := time.Unix(1000, 0)
	w := New()
	enter(w, "a", "a", n)
	enter(w, "b", "b", n)
	created := roomReq(w, "a", "create_room", "", n, false)
	if created.Room == nil || len(created.Room.Code) != 6 || created.Challenge != nil {
		t.Fatal("not waiting room", created)
	}
	joined := w.Apply(&contracts.WorldReq{UserID: "b", SessionID: "b", Action: "join_room", RoomCode: created.Room.Code}, n)
	if joined.Error != "" || len(joined.Room.Members) != 2 {
		t.Fatal(joined)
	}
	id := created.Room.ID
	roomReq(w, "a", "set_ready", id, n, true)
	roomReq(w, "b", "set_ready", id, n, true)
	started := roomReq(w, "a", "start_duo", id, n, false)
	if started.Error != "" || started.Challenge == nil {
		t.Fatal(started)
	}
	return w, n, started
}
func TestRoomPreparationOwnershipAndJoinLimits(t *testing.T) {
	n := time.Unix(1000, 0)
	w := New()
	for _, id := range []string{"a", "b", "c"} {
		enter(w, id, id, n)
	}
	created := roomReq(w, "a", "create_room", "", n, false)
	id := created.Room.ID
	if roomReq(w, "a", "start_duo", id, n, false).Error != "both_ready_required" {
		t.Fatal("started alone")
	}
	if roomReq(w, "a", "set_ready", "old", n, true).Error != "stale_room" {
		t.Fatal("stale readiness")
	}
	if roomReq(w, "a", "start_solo", "", n, false).Error != "already_in_room" {
		t.Fatal("switched mode without leaving")
	}
	for _, user := range []string{"b", "c"} {
		r := w.Apply(&contracts.WorldReq{UserID: user, SessionID: user, Action: "join_room", RoomCode: created.Room.Code}, n)
		if user == "b" && r.Error != "" {
			t.Fatal(r)
		}
		if user == "c" && r.Error != "room_full" {
			t.Fatal("third member joined", r)
		}
	}
	if len(command(w, "c", "c", "poll", n).State.Players) != 1 || command(w, "c", "c", "poll", n).Room != nil {
		t.Fatal("room leaked to outsider")
	}
	roomReq(w, "a", "set_ready", id, n, true)
	roomReq(w, "b", "set_ready", id, n, true)
	if roomReq(w, "b", "start_duo", id, n, false).Error != "host_only" {
		t.Fatal("guest started")
	}
	before := command(w, "a", "a", "poll", n)
	start := roomReq(w, "a", "start_duo", id, n, false)
	if start.Challenge.Mode != "duo" || start.Challenge.EndMS-start.Challenge.StartMS != 180000 || before.Room.Phase != "waiting" {
		t.Fatal("bad round or snapshot mutation")
	}
	if roomReq(w, "b", "set_ready", id, n, false).Error != "room_locked" {
		t.Fatal("changed active readiness")
	}
	if roomReq(w, "a", "rematch", id, n, false).Error != "round_not_finished" {
		t.Fatal("early rematch")
	}
	if w.blocked(w.players["a"], w.players["a"].state.X, w.players["a"].state.Y) || w.blocked(w.players["b"], w.players["b"].state.X, w.players["b"].state.Y) {
		t.Fatal("start body overlap")
	}
	start.Room.Members[0].Name = "forged"
	if command(w, "b", "b", "poll", n).Room.Members[0].Name == "forged" {
		t.Fatal("room snapshot aliases")
	}
}
func TestSharedInventoryRaceComboAndPracticeRelease(t *testing.T) {
	w, n, r := duo(t)
	first := r.Challenge.Items[0]
	second := r.Challenge.Items[2]
	for _, id := range []string{"a", "b"} {
		w.players[id].state.X = first.X
		w.players[id].state.Y = first.Y
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	winner := ""
	for _, id := range []string{"a", "b"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			v := w.Apply(&contracts.WorldReq{UserID: id, SessionID: id, Action: "pick", RoundID: r.Challenge.ID, ItemID: first.ID}, n)
			if v.Error == "" {
				mu.Lock()
				successes++
				winner = id
				mu.Unlock()
			}
		}(id)
	}
	wg.Wait()
	if successes != 1 {
		t.Fatal("pickup was not atomic", successes)
	}
	loser := "a"
	if winner == "a" {
		loser = "b"
	}
	w.players[loser].state.X = second.X
	w.players[loser].state.Y = second.Y
	if v := w.Apply(&contracts.WorldReq{UserID: loser, SessionID: loser, Action: "pick", RoundID: r.Challenge.ID, ItemID: second.ID}, n); v.Error != "" {
		t.Fatal(v)
	}
	if v := w.Apply(&contracts.WorldReq{UserID: loser, SessionID: loser, Action: "put", RoundID: r.Challenge.ID, ItemID: first.ID}, n); v.Error != "item_not_held" {
		t.Fatal("stole teammate hand", v)
	}
	for _, entry := range []struct{ id, item string }{{winner, first.ID}, {loser, second.ID}} {
		w.players[entry.id].state.X = 700
		w.players[entry.id].state.Y = 350
		if v := w.Apply(&contracts.WorldReq{UserID: entry.id, SessionID: entry.id, Action: "put", RoundID: r.Challenge.ID, ItemID: entry.item}, n); v.Error != "" {
			t.Fatal(v)
		}
	}
	s := command(w, "a", "a", "poll", n)
	if s.Challenge.Score != 100 || command(w, "b", "b", "poll", n).Challenge.Score != 100 {
		t.Fatal("not shared score")
	}
	total := int32(0)
	for _, m := range s.Challenge.Members {
		total += m.Pairs
	}
	if total != 1 {
		t.Fatal("contribution double counted")
	}
	item := s.Challenge.Items[0]
	for _, i := range s.Challenge.Items {
		if i.Place == "ground" && !i.Blocked {
			item = i
			break
		}
	}
	w.players["b"].state.X = item.X
	w.players["b"].state.Y = item.Y
	w.Apply(&contracts.WorldReq{UserID: "b", SessionID: "b", Action: "pick", RoundID: r.Challenge.ID, ItemID: item.ID}, n)
	roomReq(w, "b", "leave_round", r.Room.ID, n, false)
	s = command(w, "a", "a", "poll", n)
	if s.Room.Eligibility != "practice" || s.Challenge.EndMS != r.Challenge.EndMS {
		t.Fatal("quit did not practice or changed time")
	}
	found := false
	for _, i := range s.Challenge.Items {
		if i.ID == item.ID && i.Place == "ground" {
			found = true
		}
	}
	if !found {
		t.Fatal("quit orphaned held item")
	}
	if v := w.Apply(&contracts.WorldReq{UserID: "b", SessionID: "b", Action: "join_room", RoomCode: r.Room.Code}, n); v.Error != "room_locked" {
		t.Fatal("quit seat rejoined midgame", v)
	}
}
func TestReconnectPreservesRoundAndOldSessionCannotRemoveIt(t *testing.T) {
	w, n, r := duo(t)
	item := r.Challenge.Items[0]
	w.players["b"].state.X = item.X
	w.players["b"].state.Y = item.Y
	w.Apply(&contracts.WorldReq{UserID: "b", SessionID: "b", Action: "pick", RoundID: r.Challenge.ID, ItemID: item.ID}, n)
	command(w, "b", "b", "leave", n)
	s := command(w, "a", "a", "poll", n)
	if s.Room.Eligibility != "reconnecting" || len(s.State.Players) != 1 {
		t.Fatal("offline state not visible")
	}
	w.players["a"].state.X = item.X
	w.players["a"].state.Y = item.Y
	for i := 1; i <= 19; i++ {
		command(w, "a", "a", "poll", n.Add(time.Duration(i)*time.Second))
	}
	fresh := enter(w, "b", "new", n.Add(19*time.Second))
	if fresh.Challenge.ID != r.Challenge.ID || fresh.Challenge.EndMS != r.Challenge.EndMS || fresh.Room.Eligibility != "eligible" {
		t.Fatal("reconnection reset round", fresh)
	}
	held := false
	for _, i := range fresh.Challenge.Items {
		if i.ID == item.ID && i.Place == "hand" && i.OwnerID == "b" {
			held = true
		}
	}
	if !held {
		t.Fatal("reconnection lost hand")
	}
	if w.blocked(w.players["b"], w.players["b"].state.X, w.players["b"].state.Y) {
		t.Fatal("reconnection body overlap")
	}
	if command(w, "b", "b", "leave", n.Add(19*time.Second)).Error != "session_replaced_or_expired" {
		t.Fatal("old cleanup removed fresh connection")
	}
}
func TestReconnectAtDeadlineAndExpiryCleanup(t *testing.T) {
	w, n, r := duo(t)
	for i := 1; i <= 175; i++ {
		command(w, "a", "a", "poll", n.Add(time.Duration(i)*time.Second))
		command(w, "b", "b", "poll", n.Add(time.Duration(i)*time.Second))
	}
	command(w, "b", "b", "leave", n.Add(175*time.Second))
	for i := 176; i <= 180; i++ {
		command(w, "a", "a", "poll", n.Add(time.Duration(i)*time.Second))
	}
	s := command(w, "a", "a", "poll", n.Add(180*time.Second))
	if s.Challenge.Phase != "finished" || s.Room.Eligibility != "reconnecting" {
		t.Fatal("deadline shortened reconnect grace", s)
	}
	for i := 181; i <= 194; i++ {
		command(w, "a", "a", "poll", n.Add(time.Duration(i)*time.Second))
	}
	fresh := enter(w, "b", "new", n.Add(194*time.Second))
	if fresh.Room.Eligibility != "eligible" || fresh.Challenge.ID != r.Challenge.ID {
		t.Fatal("late grace reconnect failed", fresh)
	}
	w, n, r = duo(t)
	command(w, "b", "b", "leave", n)
	for i := 1; i <= 20; i++ {
		command(w, "a", "a", "poll", n.Add(time.Duration(i)*time.Second))
	}
	s = command(w, "a", "a", "poll", n.Add(20*time.Second))
	if s.Room.Eligibility != "practice" || w.memberships["b"] != "" {
		t.Fatal("expiry not applied")
	}
	hub := enter(w, "b", "new", n.Add(20*time.Second))
	if hub.Room != nil {
		t.Fatal("expired member restored")
	}
	command(w, "a", "a", "leave", n.Add(20*time.Second))
	w.Tick(n.Add(40 * time.Second))
	if len(w.rooms) != 0 || len(w.memberships) != 0 || len(w.codes) != 0 {
		t.Fatal("unowned room leaked")
	}
}
func TestBackgroundTickExpiryAndRematch(t *testing.T) {
	w, n, r := duo(t)
	for _, p := range w.players {
		p.lastSeen = n.Add(180 * time.Second)
	}
	w.Tick(n.Add(180 * time.Second))
	current := w.rooms[r.Room.ID]
	if current.phase() != "finished" || current.round.Snapshot().RemainingMS != 0 {
		t.Fatal("background did not finish")
	}
	if v := roomReq(w, "b", "rematch", r.Room.ID, n.Add(180*time.Second), false); v.Error != "host_only" {
		t.Fatal(v)
	}
	v := roomReq(w, "a", "rematch", r.Room.ID, n.Add(180*time.Second), false)
	if v.Error != "" || v.Challenge != nil || v.Room.Phase != "waiting" || v.Room.Members[0].Ready || v.Room.Members[1].Ready {
		t.Fatal("rematch did not clear readiness", v)
	}
	if v := roomReq(w, "a", "start_duo", r.Room.ID, n.Add(180*time.Second), false); v.Error != "both_ready_required" {
		t.Fatal("rematch skipped preparation", v)
	}
	roomReq(w, "a", "leave_round", r.Room.ID, n.Add(180*time.Second), false)
	v = command(w, "b", "b", "poll", n.Add(180*time.Second))
	if v.Room.HostID != "b" || len(v.Room.Members) != 1 {
		t.Fatal("host not transferred", v)
	}
}
