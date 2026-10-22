// 文件说明：验证 NPC 三维距离、私有委托状态、并发唯一奖励与会话重置。
package world

import (
	"game-shelf/pkg/contracts"
	"math"
	"sync"
	"testing"
	"time"
)

func npcCommand(w *World, id, action, npc, choice string, now time.Time) *contracts.WorldResp {
	return w.Apply(&contracts.WorldReq{UserID: id, SessionID: id, Action: action, NPCID: npc, ChoiceID: choice}, now)
}
func atKeeper(w *World, id string) {
	w.players[id].state.X, w.players[id].state.Y, w.players[id].state.Z = 540, 550, 0
}
func collectForQuest(t *testing.T, w *World, id string, now time.Time) {
	t.Helper()
	n := w.nodes[0]
	w.players[id].state.X, w.players[id].state.Y = n.X-50, n.Y
	if r := command(w, id, id, "gather", now); r.Error != "" {
		t.Fatal(r.Error)
	}
}
func TestNPCPlacementHasRoomForItsBody(t *testing.T) {
	w := New()
	for _, npc := range w.layout.NPCs {
		for _, e := range w.layout.Entities {
			if e.ID != npc.ID && overlaps(npc.X, npc.Y, e) {
				t.Fatal("NPC placed inside another entity", npc.ID, e.ID)
			}
		}
	}
}

func TestNPCDistanceChoicesAndSolids(t *testing.T) {
	w, now := New(), time.Now()
	enter(w, "a", "a", now)
	for _, tc := range []struct{ npc, action, choice, want string }{
		{"missing", "interact", "", "unknown_npc"},
		{"keeper", "interact", "", "npc_out_of_range"},
		{"keeper", "choose", "accept", "npc_out_of_range"},
	} {
		if r := npcCommand(w, "a", tc.action, tc.npc, tc.choice, now); r.Error != tc.want {
			t.Fatal(r)
		}
	}
	atKeeper(w, "a")
	for _, tc := range []struct{ action, choice, want string }{
		{"choose", "", "invalid_npc_choice"},
		{"choose", "reward_999", "invalid_npc_choice"},
		{"interact", "accept", "invalid_npc_choice"},
		{"choose", "complete", "quest_not_ready"},
	} {
		if r := npcCommand(w, "a", tc.action, "keeper", tc.choice, now); r.Error != tc.want {
			t.Fatal(r)
		}
	}
	p := w.players["a"]
	p.state.X, p.state.Y, p.state.Z = 540, 500, 86
	if r := npcCommand(w, "a", "interact", "keeper", "", now); r.Error != "npc_out_of_range" {
		t.Fatal("height ignored", r)
	}
	p.state.Z = 85
	if r := npcCommand(w, "a", "interact", "keeper", "", now); r.Dialog == nil {
		t.Fatal("boundary rejected", r)
	}
	for _, n := range w.layout.NPCs {
		p.state.X, p.state.Y, p.state.Z = n.X-60, n.Y, 0
		if !w.blocked(p, n.X, n.Y) {
			t.Fatal("NPC not solid", n.ID)
		}
		if r := npcCommand(w, "a", "interact", n.ID, "", now); r.Dialog == nil || len(r.Dialog.Choices) == 0 {
			t.Fatal(r)
		}
	}
	for _, choice := range []string{"camp", "crystals"} {
		if r := npcCommand(w, "a", "choose", "ranger", choice, now); r.Dialog == nil {
			t.Fatal(r)
		}
	}
	if r := npcCommand(w, "a", "choose", "ranger", "accept", now); r.Error != "invalid_npc_choice" {
		t.Fatal(r)
	}
}
func TestQuestRequiresNewGatherAndRewardsOnce(t *testing.T) {
	w, now := New(), time.Now()
	enter(w, "a", "a", now)
	enter(w, "b", "b", now)
	collectForQuest(t, w, "a", now)
	atKeeper(w, "a")
	r := npcCommand(w, "a", "choose", "keeper", "accept", now)
	if r.Quest.Status != "active" || r.Quest.Progress != 0 {
		t.Fatal("retroactive progress", r)
	}
	if r := npcCommand(w, "a", "choose", "keeper", "complete", now); r.Error != "quest_not_ready" {
		t.Fatal(r)
	}
	// 使用可控时钟推进重生，同时续租，不能因测试跨十秒被世界移除。
	for i := 1; i <= 3; i++ {
		for j := 1; j <= 4; j++ {
			command(w, "a", "a", "poll", now.Add(time.Duration(j)*3*time.Second))
		}
		now = now.Add(12 * time.Second)
		collectForQuest(t, w, "a", now)
		atKeeper(w, "a")
		r = npcCommand(w, "a", "choose", "keeper", "accept", now)
		if r.Quest.Progress != int32(math.Min(float64(i), 2)) {
			t.Fatal("progress reset or overflow", r)
		}
	}
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() { defer wg.Done(); npcCommand(w, "a", "choose", "keeper", "complete", now) }()
	}
	wg.Wait()
	r = command(w, "a", "a", "poll", now)
	if w.players["a"].state.Score != 7 || r.Quest.Status != "completed" || r.Quest.Progress != 2 {
		t.Fatal("not unique reward", r)
	}
	r.Quest.Progress = 99
	if command(w, "a", "a", "poll", now).Quest.Progress != 2 {
		t.Fatal("shared private response")
	}
	if r.Dialog != nil {
		t.Fatal("poll leaked dialog")
	}
	enter(w, "b", "b", now)
	peer := command(w, "b", "b", "poll", now)
	if peer.Quest.Progress != 0 || peer.Quest.Status != "" || peer.Dialog != nil {
		t.Fatal("private quest leaked")
	}
	atKeeper(w, "a")
	if r := npcCommand(w, "a", "choose", "keeper", "accept", now); r.Quest.Status != "completed" {
		t.Fatal("completed quest restarted")
	}
}
func TestQuestResetAndSessionProtection(t *testing.T) {
	for _, mode := range []string{"leave", "expire", "replace"} {
		t.Run(mode, func(t *testing.T) {
			w, now := New(), time.Now()
			enter(w, "a", "a", now)
			atKeeper(w, "a")
			npcCommand(w, "a", "choose", "keeper", "accept", now)
			collectForQuest(t, w, "a", now)
			if mode == "leave" {
				command(w, "a", "a", "leave", now)
			}
			if mode == "expire" {
				now = now.Add(6 * time.Second)
			}
			session := "a"
			if mode == "replace" {
				session = "new"
			}
			r := enter(w, "a", session, now)
			if r.Quest.Progress != 0 || r.Quest.Status != "" || w.players["a"].state.Score != 0 {
				t.Fatal("quest survived session", r)
			}
			if mode == "replace" {
				if r := npcCommand(w, "a", "choose", "keeper", "complete", now); r.Error != "session_replaced_or_expired" {
					t.Fatal("old session changed quest", r)
				}
			}
		})
	}
}
