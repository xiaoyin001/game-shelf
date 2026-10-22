// 文件说明：验证权威出生位置在边界内且不完全重叠，保留首位旅人的旧起点。
package world

import (
	"fmt"
	"math"
	"testing"
	"time"
)

func TestAuthoritativeSpawnSeparation(t *testing.T) {
	w := New()
	now := time.Now()
	for i := range 100 {
		id := fmt.Sprint("player-", i)
		reply := enter(w, id, id, now)
		if reply.Error != "" {
			t.Fatal(reply.Error)
		}
		if i == 0 && (reply.State.Players[0].X != 560 || reply.State.Players[0].Y != 400) {
			t.Fatal("first spawn changed")
		}
		for _, a := range reply.State.Players {
			if a.X < 20 || a.X > width-20 || a.Y < 20 || a.Y > height-20 {
				t.Fatal("out of world")
			}
			for _, b := range reply.State.Players {
				if a.ID != b.ID && math.Hypot(a.X-b.X, a.Y-b.Y) < 28 {
					t.Fatal("overlapping spawn")
				}
			}
		}
	}
}
