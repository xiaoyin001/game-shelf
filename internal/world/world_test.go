// 文件说明：验证在线角色进入、会话保护、过期与独立快照，不把未实现玩法写为通过。
package world

import (
	"game-shelf/pkg/contracts"
	"testing"
	"time"
)

func enter(w *World, id, session string, now time.Time) *contracts.WorldResp {
	return w.Apply(&contracts.WorldReq{UserID: id, SessionID: session, Name: id, Action: "enter"}, now)
}
func command(w *World, id, session, action string, now time.Time) *contracts.WorldResp {
	return w.Apply(&contracts.WorldReq{UserID: id, SessionID: session, Action: action}, now)
}
func TestPresenceAndLease(t *testing.T) {
	now := time.Unix(1000, 0)
	w := New()
	old := enter(w, "a", "old", now)
	enter(w, "b", "b", now)
	if len(command(w, "b", "b", "poll", now).State.Players) != 2 {
		t.Fatal("not shared")
	}
	enter(w, "a", "new", now)
	if command(w, "a", "old", "leave", now).Error == "" {
		t.Fatal("old session can remove new player")
	}
	if len(command(w, "a", "new", "poll", now).State.Players) != 2 {
		t.Fatal("new player removed")
	}
	command(w, "b", "b", "leave", now)
	if len(old.State.Players) != 1 {
		t.Fatal("snapshot was mutated")
	}
	if command(w, "a", "new", "poll", now.Add(6*time.Second)).Error == "" {
		t.Fatal("lease did not expire")
	}
}
