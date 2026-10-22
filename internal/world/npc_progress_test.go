// 文件说明：确认只有本玩家成功的采集才推进已领取委托，冷却、远程和其他玩家采集不能代领。
package world

import (
	"testing"
	"time"
)

func TestQuestCountsOnlySuccessfulSelfGather(t *testing.T) {
	w, now := New(), time.Now()
	for _, id := range []string{"a", "b"} {
		enter(w, id, id, now)
		atKeeper(w, id)
		if r := npcCommand(w, id, "choose", "keeper", "accept", now); r.Quest.Status != "active" {
			t.Fatal(r)
		}
	}
	collectForQuest(t, w, "a", now)
	w.players["b"].state.X, w.players["b"].state.Y = 670, 400
	if r := command(w, "b", "b", "gather", now); r.Error != "no_resource_in_range" {
		t.Fatal("shared cooldown ignored", r)
	}
	w.players["b"].state.X, w.players["b"].state.Y = 1000, 700
	if r := command(w, "b", "b", "gather", now); r.Error != "no_resource_in_range" {
		t.Fatal("remote gathering accepted", r)
	}
	if w.players["a"].quest.Progress != 1 || w.players["b"].quest.Progress != 0 {
		t.Fatal("failed or other player's gather counted")
	}
	atKeeper(w, "b")
	if r := npcCommand(w, "b", "choose", "keeper", "complete", now); r.Error != "quest_not_ready" || w.players["b"].state.Score != 0 {
		t.Fatal("another player's quest redeemed", r)
	}
}
