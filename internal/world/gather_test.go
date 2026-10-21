// 文件说明：验证共享采集的并发原子性、距离/重生和角色防护容量。
package world

import (
	"fmt"
	"game-shelf/pkg/contracts"
	"sync"
	"testing"
	"time"
)

func TestGatherIsSharedAtomicAndDistanceChecked(t *testing.T) {
	now := time.Unix(1000, 0)
	w := New()
	enter(w, "a", "a", now)
	enter(w, "b", "b", now)
	var wg sync.WaitGroup
	for _, id := range []string{"a", "b"} {
		wg.Add(1)
		go func(id string) { defer wg.Done(); command(w, id, id, "gather", now) }(id)
	}
	wg.Wait()
	r := command(w, "a", "a", "poll", now)
	if r.State.Players[0].Score+r.State.Players[1].Score != 1 {
		t.Fatal("duplicate reward")
	}
	if command(w, "a", "a", "gather", now).Error == "" {
		t.Fatal("cooldown bypass")
	}
	for i := 1; i <= 15; i++ {
		w.Apply(&contracts.WorldReq{UserID: "a", SessionID: "a", Action: "input", DX: -1}, now.Add(time.Duration(i)*100*time.Millisecond))
	}
	if command(w, "a", "a", "gather", now.Add(2*time.Second)).Error == "" {
		t.Fatal("remote gather allowed")
	}
	// 保持租约存活，再验证共享资源十秒重生。
	for i := 1; i <= 10; i++ {
		command(w, "b", "b", "poll", now.Add(time.Duration(i)*time.Second))
	}
	if command(w, "b", "b", "gather", now.Add(10*time.Second)).Error != "" {
		t.Fatal("resource did not respawn")
	}
}
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
