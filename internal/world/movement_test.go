// 文件说明：验证真实服务器时间预算下的移动、方向与地图边界。
package world

import (
	"game-shelf/pkg/contracts"
	"math"
	"testing"
	"time"
)

func TestMovementIsServerAuthoritative(t *testing.T) {
	now := time.Unix(1000, 0)
	w := New()
	initial := enter(w, "a", "s", now)
	moved := w.Apply(&contracts.WorldReq{UserID: "a", SessionID: "s", Action: "input", DX: 100, DY: 100}, now.Add(100*time.Millisecond))
	p := moved.State.Players[0]
	if math.Abs(math.Hypot(p.X-560, p.Y-400)-18) > 0.001 {
		t.Fatalf("diagonal speed: %+v", p)
	}
	if initial.State.Players[0].X != 560 {
		t.Fatal("snapshot mutated")
	}
	// 高频请求不增加总位移，长时间空闲也不能换来瞬移。
	for range 100 {
		w.Apply(&contracts.WorldReq{UserID: "a", SessionID: "s", Action: "input", DX: 1}, now.Add(100*time.Millisecond))
	}
	r := w.Apply(&contracts.WorldReq{UserID: "a", SessionID: "s", Action: "input", DX: 1}, now.Add(2*time.Second))
	if math.Abs(r.State.Players[0].X-p.X-27) > 0.001 {
		t.Fatal("idle movement budget exceeded")
	}
	bad := w.Apply(&contracts.WorldReq{UserID: "a", SessionID: "s", Action: "input", DX: math.NaN()}, now.Add(2*time.Second))
	if bad.Error != "invalid_direction" {
		t.Fatal(bad)
	}
	for i := 21; i <= 1000; i++ {
		w.Apply(&contracts.WorldReq{UserID: "a", SessionID: "s", Action: "input", DX: 1, DY: -1}, now.Add(time.Duration(i)*100*time.Millisecond))
	}
	r = command(w, "a", "s", "poll", now.Add(100*time.Second))
	p = r.State.Players[0]
	if p.X > 1180 || p.Y < 20 {
		t.Fatal("map bounds violated")
	}
}
