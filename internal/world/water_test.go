// 文件说明：验证服务端浅水速度、边界、跳跃与洪泛预算，测试夹具不暴露坐标设置接口。
package world

import (
	"game-shelf/pkg/contracts"
	"game-shelf/pkg/scene"
	"math"
	"testing"
	"time"
)

func TestWaterEllipseUsesSharedGeometry(t *testing.T) {
	w := scene.Default().Water
	if w.X != 930 || w.Y != 465 || w.RadiusX != 92 || w.RadiusY != 59.8 || w.SpeedFactor != .55 {
		t.Fatal("water drift", w)
	}
	for _, p := range []scene.Point{{X: w.X, Y: w.Y}, {X: w.X + w.RadiusX, Y: w.Y}, {X: w.X, Y: w.Y + w.RadiusY - .000001}} {
		if !w.Contains(p.X, p.Y) {
			t.Fatal("water boundary missing", p)
		}
	}
	for _, p := range []scene.Point{{X: w.X + w.RadiusX + .001, Y: w.Y}, {X: w.X + 70, Y: w.Y + 50}, {X: math.NaN(), Y: w.Y}, {X: w.X, Y: math.Inf(1)}} {
		if w.Contains(p.X, p.Y) {
			t.Fatal("outside considered water", p)
		}
	}
	if (scene.Water{}).Contains(0, 0) {
		t.Fatal("empty water enabled")
	}
}
func TestWaterSpeedDiagonalFloodAndLargeIdleBudget(t *testing.T) {
	for _, direction := range [][2]float64{{1, 0}, {1, 1}} {
		now := time.Unix(1000, 0)
		w := emptyPhysics(now)
		p := w.players["a"]
		p.state.X, p.state.Y = 930, 465
		initial := w.response(p, now)
		r := stepInput(w, &now, direction[0], direction[1])
		if math.Abs(math.Hypot(p.state.X-930, p.state.Y-465)-9.9) > .000001 || !r.State.Players[0].Wading {
			t.Fatal("not 55 percent", p.state)
		}
		before := p.state
		for range 100 {
			w.Apply(&contracts.WorldReq{UserID: "a", SessionID: "a", Action: "input", DX: direction[0], DY: direction[1]}, now)
		}
		if p.state != before {
			t.Fatal("flood bypassed water budget")
		}
		now = now.Add(2 * time.Second)
		w.Apply(&contracts.WorldReq{UserID: "a", SessionID: "a", Action: "input", DX: direction[0], DY: direction[1]}, now)
		if math.Abs(math.Hypot(p.state.X-before.X, p.state.Y-before.Y)-14.85) > .000001 {
			t.Fatal("idle stored water movement")
		}
		if !initial.State.Players[0].Wading || initial.State.Players[0].X != 930 {
			t.Fatal("snapshot mutation")
		}
	}
}
func TestWaterEntryExitJumpLandingAndRaisedSupport(t *testing.T) {
	now := time.Unix(1000, 0)
	w := emptyPhysics(now)
	p := w.players["a"]
	p.state.X, p.state.Y = 830, 465
	stepInput(w, &now, 1, 0)
	if !w.wading(p) || p.state.X >= 848 || p.state.X <= 839 {
		t.Fatal("entry did not slow within packet", p.state)
	}
	p.state.X = 1019
	before := p.state.X
	stepInput(w, &now, 1, 0)
	if w.wading(p) || p.state.X-before < 14 {
		t.Fatal("exit stayed slowed", p.state)
	}
	p.state.X, p.state.Y = 930, 465
	if r := command(w, "a", "a", "jump", now); r.Error != "" || r.State.Players[0].Wading {
		t.Fatal("jump still wading")
	}
	stepInput(w, &now, 1, 0)
	if math.Abs(p.state.X-948) > .000001 || w.wading(p) {
		t.Fatal("air speed changed")
	}
	r := command(w, "a", "a", "poll", now.Add(time.Second))
	if !r.State.Players[0].Wading || !p.state.Grounded {
		t.Fatal("landing not slowed")
	}
	w.layout.Entities = []scene.Entity{{X: p.state.X, Y: p.state.Y, Radius: 30, Height: 6}}
	p.state.Z = 6
	if w.response(p, now).State.Players[0].Wading {
		t.Fatal("solid above water slowed")
	}
}
func TestWaterCrossingPacketFrequencyAndPeerSnapshot(t *testing.T) {
	distance := func(interval time.Duration) float64 {
		now := time.Unix(1000, 0)
		w := emptyPhysics(now)
		p := w.players["a"]
		p.state.X, p.state.Y = 820, 465
		for elapsed := interval; elapsed <= 3*time.Second; elapsed += interval {
			w.Apply(&contracts.WorldReq{UserID: "a", SessionID: "a", Action: "input", DX: 1}, now.Add(elapsed))
		}
		return p.state.X
	}
	if math.Abs(distance(10*time.Millisecond)-distance(100*time.Millisecond)) > 3 {
		t.Fatal("packet frequency accelerated crossing")
	}
	now := time.Unix(1000, 0)
	w := emptyPhysics(now)
	enter(w, "b", "b", now)
	w.players["a"].state.X, w.players["a"].state.Y = 930, 465
	r := command(w, "b", "b", "poll", now)
	for _, p := range r.State.Players {
		if p.Wading != (p.ID == "a") {
			t.Fatal("peer sees different terrain", p)
		}
	}
}
