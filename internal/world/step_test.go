// 文件说明：验证低台阶的相对步高、顶面支撑、净空与反空中爬升，夹具不改变运行场景。
package world

import (
	"fmt"
	"game-shelf/pkg/contracts"
	"game-shelf/pkg/scene"
	"math"
	"testing"
	"time"
)

func stepInput(w *World, now *time.Time, dx, dy float64) *contracts.WorldResp {
	*now = now.Add(100 * time.Millisecond)
	return w.Apply(&contracts.WorldReq{UserID: "a", SessionID: "a", Action: "input", DX: dx, DY: dy}, *now)
}

func TestStepHeightThresholdForCircleAndBox(t *testing.T) {
	for _, shape := range []string{"circle", "box"} {
		for _, height := range []float64{6, stepHeight - .001, stepHeight, stepHeight + .001, 50, 120} {
			t.Run(fmt.Sprintf("%s/height=%.3f", shape, height), func(t *testing.T) {
				now := time.Unix(1000, 0)
				w := emptyPhysics(now)
				e := scene.Entity{ID: "step", Kind: "rock", X: 620, Y: 400, Radius: 22, Height: height}
				if shape == "box" {
					e.Radius, e.HalfX, e.HalfY = 0, 22, 30
				}
				w.layout.Entities = []scene.Entity{e}
				for range 3 {
					stepInput(w, &now, 1, 0)
				}
				p := w.players["a"]
				if height <= stepHeight {
					if p.state.X <= 600 || p.state.Z != height || !p.state.Grounded || !w.supported(p) {
						t.Fatal("did not stand on low top", p.state)
					}
					command(w, "a", "a", "poll", now.Add(time.Second))
					if p.state.Z != height || !p.state.Grounded {
						t.Fatal("idle did not retain support")
					}
				} else if p.state.X > 578+epsilon || p.state.Z != 0 {
					t.Fatal("climbed high obstacle", p.state)
				}
				if w.blocked(p, p.state.X, p.state.Y) {
					t.Fatal("walked inside solid", p.state)
				}
			})
		}
	}
}

func TestStairsUseRelativeHeightThenJumpAndWalkOff(t *testing.T) {
	now := time.Unix(1000, 0)
	w := emptyPhysics(now)
	for i := 1; i <= 3; i++ {
		w.layout.Entities = append(w.layout.Entities, scene.Entity{Kind: "rock", X: 570 + float64(i)*50, Y: 400, HalfX: 25, HalfY: 35, Height: float64(i) * stepHeight})
	}
	for range 7 {
		stepInput(w, &now, 1, 0)
		if w.blocked(w.players["a"], w.players["a"].state.X, 400) {
			t.Fatal("stair penetration")
		}
	}
	p := w.players["a"]
	if p.state.Z != 3*stepHeight || !p.state.Grounded {
		t.Fatal("did not climb consecutive steps", p.state)
	}
	if p.state.X > 560+7*speed*.1+epsilon {
		t.Fatal("step added horizontal speed")
	}
	if r := command(w, "a", "a", "jump", now); r.Error != "" {
		t.Fatal(r.Error)
	}
	command(w, "a", "a", "poll", now.Add(475*time.Millisecond))
	if math.Abs(p.state.Z-(3*stepHeight+90.25)) > .001 {
		t.Fatal("jump from step not authoritative", p.state)
	}
	now = now.Add(time.Second)
	command(w, "a", "a", "poll", now)
	if p.state.Z != 3*stepHeight || !p.state.Grounded {
		t.Fatal("did not land back on top")
	}
	for range 10 {
		stepInput(w, &now, 1, 0)
	}
	command(w, "a", "a", "poll", now.Add(time.Second))
	if p.state.Z != 0 || !p.state.Grounded {
		t.Fatal("floating after stepping off", p.state)
	}
}

func TestStepRejectsAirIdleAndFlood(t *testing.T) {
	now := time.Unix(1000, 0)
	for _, mode := range []string{"jumping", "falling", "unsupported", "idle", "flood"} {
		t.Run(mode, func(t *testing.T) {
			w := emptyPhysics(now)
			w.layout.Entities = []scene.Entity{{Kind: "rock", X: 620, Y: 400, Radius: 22, Height: stepHeight}}
			p := w.players["a"]
			p.state.X = 578
			switch mode {
			case "jumping":
				p.state.Grounded, p.vz = false, jumpSpeed
			case "falling":
				p.state.Grounded, p.state.Z, p.vz = false, 6, -1
			case "unsupported":
				p.state.Z = 6
			case "idle":
				stepInput(w, &now, 0, 0)
				if p.state.Z != 0 {
					t.Fatal("idle raised feet")
				}
				return
			case "flood":
				for range 100 {
					w.Apply(&contracts.WorldReq{UserID: "a", SessionID: "a", Action: "input", DX: 1}, now)
				}
				if p.state.X != 578 || p.state.Z != 0 {
					t.Fatal("zero-time flood moved player")
				}
				return
			}
			before, vz := p.state, p.vz
			if w.walk(p, 581, 400) || p.state != before || p.vz != vz {
				t.Fatal("air step or partial mutation", p.state)
			}
		})
	}
}

func TestStepChecksLiftAndDestinationHeadroom(t *testing.T) {
	for _, overheadX := range []float64{578, 619} {
		now := time.Unix(1000, 0)
		w := emptyPhysics(now)
		enter(w, "b", "b", now)
		w.layout.Entities = []scene.Entity{{Kind: "rock", X: 620, Y: 400, Radius: 22, Height: stepHeight}}
		p, above := w.players["a"], w.players["b"]
		p.state.X = 578
		above.state.X, above.state.Y, above.state.Z = overheadX, 400, 75
		before := p.state
		if w.walk(p, 581, 400) || p.state != before {
			t.Fatal("stepped into overhead player", overheadX, p.state)
		}
		delete(w.players, "b")
		if !w.walk(p, 581, 400) || p.state.Z != stepHeight {
			t.Fatal("headroom did not reopen")
		}
	}
}

func TestStepCannotBypassHighWallActorsOrCeiling(t *testing.T) {
	now := time.Unix(1000, 0)
	w := emptyPhysics(now)
	w.layout.Entities = []scene.Entity{{Kind: "rock", X: 620, Y: 400, Radius: 22, Height: 18}, {Kind: "tree", X: 640, Y: 400, Radius: 24, Height: 120}}
	for range 15 {
		stepInput(w, &now, 1, 0)
	}
	p := w.players["a"]
	if p.state.X > 596+epsilon || p.state.Z != 18 || w.blocked(p, p.state.X, p.state.Y) {
		t.Fatal("low prop bypassed high wall", p.state)
	}
	w = emptyPhysics(now)
	p = w.players["a"]
	p.state.X = 578
	w.layout.Entities = []scene.Entity{{Kind: "npc", X: 620, Y: 400, Radius: 22, Height: 12}}
	if w.walk(p, 581, 400) {
		t.Fatal("NPC treated as step")
	}
	w = emptyPhysics(now)
	p = w.players["a"]
	p.state.X, p.state.Z = 578, 438
	w.layout.Entities = []scene.Entity{{X: 538, Y: 400, HalfX: 40, HalfY: 30, Height: 438}, {X: 620, Y: 400, Radius: 22, Height: 450}}
	if w.walk(p, 581, 400) || p.state.Z != 438 {
		t.Fatal("step exceeded world ceiling")
	}
}

func TestLeavingSupportDoesNotStepAcrossGapInSameInput(t *testing.T) {
	now := time.Unix(1000, 0)
	w := emptyPhysics(now)
	w.layout.Entities = []scene.Entity{{X: 540, Y: 400, HalfX: 20, HalfY: 30, Height: 18}, {X: 625, Y: 400, Radius: 22, Height: 36}}
	p := w.players["a"]
	p.state.X, p.state.Z = 578, 18
	stepInput(w, &now, 1, 0)
	if p.state.Grounded || p.state.Z != 18 || p.state.X >= 583 {
		t.Fatal("stepped in midair after leaving platform", p.state)
	}
}

func TestDiagonalStepRetainsSolidAndSpeedLimits(t *testing.T) {
	now := time.Unix(1000, 0)
	w := emptyPhysics(now)
	w.layout.Entities = []scene.Entity{{Kind: "rock", X: 620, Y: 400, Radius: 30, Height: 8}}
	seen := false
	for range 5 {
		stepInput(w, &now, 1, 1)
		p := w.players["a"]
		seen = seen || p.state.Z == 8
		if w.blocked(p, p.state.X, p.state.Y) || math.Hypot(p.state.X-560, p.state.Y-400) > speed*.5+epsilon {
			t.Fatal("invalid diagonal step", p.state)
		}
	}
	if !seen {
		t.Fatal("did not step while moving diagonally")
	}
}
