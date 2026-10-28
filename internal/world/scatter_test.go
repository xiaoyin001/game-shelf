// 文件说明：验证分散锚点的全部抖动角落与原NPC位置均真实可步行，不注入运行时坐标。
package world

import (
	"game-shelf/pkg/contracts"
	"game-shelf/pkg/scene"
	"game-shelf/tests/navigation"
	"math"
	"testing"
	"time"
)

func TestScatterAnchorsAndRetiredNPCGroundReachable(t *testing.T) {
	l := scene.Default()
	zones := [8]int{}
	for _, a := range l.Workshop.Spots {
		if a.Zone < 0 || a.Zone >= 8 {
			t.Fatal("invalid zone", a)
		}
		zones[a.Zone]++
		// 三角不等式留白覆盖整个抖动方框，不只验证四个角。
		padding := playerRadius + math.Sqrt2*scene.ScatterJitter
		for _, e := range l.Entities {
			if e.Radius > 0 {
				if math.Hypot(a.X-e.X, a.Y-e.Y) < padding+e.Radius {
					t.Fatal("insufficient scatter padding", a, e)
				}
			} else {
				x := math.Max(e.X-e.HalfX, math.Min(e.X+e.HalfX, a.X))
				y := math.Max(e.Y-e.HalfY, math.Min(e.Y+e.HalfY, a.Y))
				if math.Hypot(a.X-x, a.Y-y) < padding {
					t.Fatal("insufficient box padding", a, e)
				}
			}
		}
		if math.Hypot(a.X-l.Workshop.X, a.Y-l.Workshop.Y) < 160 {
			t.Fatal("anchor still around table", a)
		}
		for _, dx := range []float64{-scene.ScatterJitter, scene.ScatterJitter} {
			for _, dy := range []float64{-scene.ScatterJitter, scene.ScatterJitter} {
				now := time.Unix(1000, 0)
				w := New()
				enter(w, "a", "s", now)
				p := w.players["a"]
				target := scene.Point{X: a.X + dx, Y: a.Y + dy}
				if w.blocked(p, target.X, target.Y) {
					t.Fatal("jitter inside collider", target)
				}
				testWalkRoute(t, w, p, target, &now)
			}
		}
	}
	for _, count := range zones {
		if count < 2 {
			t.Fatal("zone has no random choice", zones)
		}
	}
	if l.Workshop.X != 700 || l.Workshop.Y != 350 || l.Workshop.Range != 90 {
		t.Fatal("table moved")
	}
	for _, target := range []scene.Point{{X: 540, Y: 500}, {X: 700, Y: 230}} {
		now := time.Unix(1000, 0)
		w := New()
		enter(w, "a", "s", now)
		p := w.players["a"]
		if w.blocked(p, target.X, target.Y) {
			t.Fatal("invisible retired NPC collider")
		}
		testWalkRoute(t, w, p, target, &now)
	}
}

func testWalkRoute(t *testing.T, w *World, p *player, target scene.Point, now *time.Time) {
	t.Helper()
	path := navigation.Route(w.layout, scene.Point{X: p.state.X, Y: p.state.Y}, target, nil)
	if len(path) == 0 {
		t.Fatal("no ground path", target)
	}
	for _, next := range path {
		reached := false
		for range 1000 {
			dx, dy := next.X-p.state.X, next.Y-p.state.Y
			length := math.Hypot(dx, dy)
			if length < 2 {
				reached = true
				break
			}
			*now = now.Add(100 * time.Millisecond)
			scale := math.Max(18, length)
			r := w.Apply(&contracts.WorldReq{UserID: "a", SessionID: "s", Action: "input", DX: dx / scale, DY: dy / scale}, *now)
			if r.Error != "" || w.blocked(p, p.state.X, p.state.Y) {
				t.Fatal("invalid walk", r.Error, p.state)
			}
		}
		if !reached {
			t.Fatal("walk stuck", p.state, next, target)
		}
	}
	if math.Hypot(p.state.X-target.X, p.state.Y-target.Y) > 32 {
		t.Fatal("goal not in pickup range")
	}
}
