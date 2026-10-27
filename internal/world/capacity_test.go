// 文件说明：保留已下线采集测试中的身份、租约和容量保护回归。
package world

import (
	"fmt"
	"testing"
	"time"
)

func TestSessionReplacementExpiryAndCapacity(t *testing.T) {
	now := time.Unix(1000, 0)
	w := New()
	if command(w, "a", "s", "poll", now).Error == "" {
		t.Fatal("unauthenticated access")
	}
	enter(w, "a", "old", now)
	enter(w, "a", "new", now)
	if command(w, "a", "old", "leave", now).Error == "" {
		t.Fatal("old session removed new player")
	}
	if len(command(w, "a", "new", "poll", now).State.Players) != 1 {
		t.Fatal("new session lost")
	}
	if command(w, "a", "new", "poll", now.Add(6*time.Second)).Error == "" {
		t.Fatal("expired player remained")
	}
	for i := 0; i < 100; i++ {
		id := fmt.Sprint(i)
		if enter(w, id, id, now).Error != "" {
			t.Fatal("unexpected capacity")
		}
	}
	if enter(w, "overflow", "overflow", now).Error != "world_full" {
		t.Fatal("world cap bypass")
	}
}
