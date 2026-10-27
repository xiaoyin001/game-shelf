// 文件说明：验证权威挑战的会话、作用域、旧入口关闭及并发去重。
package world

import (
	"game-shelf/pkg/contracts"
	"sync"
	"testing"
	"time"
)

func TestLeaveSoloFindsFreeHubBody(t *testing.T) {
	n := time.Unix(1000, 0)
	w := New()
	enter(w, "a", "a", n)
	enter(w, "b", "b", n)
	command(w, "a", "a", "start_solo", n)
	p := w.players["a"]
	other := w.players["b"]
	other.state.X, other.state.Y = p.state.X, p.state.Y
	if r := command(w, "a", "a", "leave_round", n); r.Error != "" || r.Challenge != nil || len(r.State.Players) != 2 {
		t.Fatal("not back in hub", r)
	}
	if w.blocked(p, p.state.X, p.state.Y) {
		t.Fatal("return overlapped a hub player")
	}
}

func TestSoloScopePhysicsAndOldEntryRetired(t *testing.T) {
	n := time.Unix(1000, 0)
	w := New()
	enter(w, "a", "a", n)
	enter(w, "b", "b", n)
	a := command(w, "a", "a", "start_solo", n)
	b := command(w, "b", "b", "start_solo", n)
	if a.Challenge.ID == b.Challenge.ID || len(a.State.Players) != 1 || len(b.State.Players) != 1 {
		t.Fatal("rounds not isolated")
	}
	if w.blocked(w.players["a"], w.players["b"].state.X, w.players["b"].state.Y) {
		t.Fatal("invisible unrelated player body")
	}
	i := a.Challenge.Items[0]
	w.players["a"].state.X = i.X
	w.players["a"].state.Y = i.Y
	req := &contracts.WorldReq{UserID: "a", SessionID: "a", Action: "pick", RoundID: a.Challenge.ID, ItemID: i.ID}
	successes := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if w.Apply(req, n).Error == "" {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if successes != 1 {
		t.Fatal("duplicate pickup", successes)
	}
	for _, action := range []string{"gather", "explore", "decorate", "interact", "choose"} {
		if command(w, "a", "a", action, n).Error != "unknown_action" {
			t.Fatal("retired entry", action)
		}
	}
	if command(w, "a", "old", "start_solo", n).Error != "session_replaced_or_expired" {
		t.Fatal("old session can mutate")
	}
	w.Apply(&contracts.WorldReq{UserID: "a", SessionID: "a", Action: "chat", Text: "单人私语"}, n)
	if len(command(w, "b", "b", "poll", n).State.Chats) != 0 || len(command(w, "a", "a", "poll", n).State.Chats) != 1 {
		t.Fatal("chat leaked across independent rounds")
	}
	if command(w, "a", "a", "start_solo", n).Error != "already_in_round" {
		t.Fatal("restart before time up")
	}
}
