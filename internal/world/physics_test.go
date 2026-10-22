// 文件说明：覆盖服务端跳跃轨迹、实体支撑、玩家互撞和碰撞分步；测试夹具不改变运行场景。
package world

import (
	"game-shelf/pkg/contracts"
	"game-shelf/pkg/scene"
	"math"
	"math/rand"
	"testing"
	"time"
)

func emptyPhysics(now time.Time) *World {
	w := New()
	w.layout.Entities = nil
	enter(w, "a", "a", now)
	return w
}
func TestJumpTrajectoryAndNoAirJump(t *testing.T) {
	now := time.Unix(1000, 0)
	w := emptyPhysics(now)
	original := command(w, "a", "a", "poll", now)
	if r := command(w, "a", "a", "jump", now); r.Error != "" || r.State.Players[0].Grounded {
		t.Fatal("no jump")
	}
	for i := range 50 {
		if r := command(w, "a", "a", "jump", now.Add(200*time.Millisecond)); r.Error != "jump_not_grounded" {
			t.Fatal("air jump", i)
		}
	}
	r := command(w, "a", "a", "poll", now.Add(475*time.Millisecond))
	if math.Abs(r.State.Players[0].Z-90.25) > .001 {
		t.Fatal("trajectory altered by message flood", r.State.Players[0])
	}
	r = command(w, "a", "a", "poll", now.Add(time.Second))
	if r.State.Players[0].Z != 0 || !r.State.Players[0].Grounded || original.State.Players[0].Z != 0 {
		t.Fatal("no input did not land or snapshot mutated")
	}
	if command(w, "a", "a", "jump", now.Add(time.Second)).Error != "" {
		t.Fatal("cannot jump after landing")
	}
}
func TestAllPhysicalShapesAndSliding(t *testing.T) {
	now := time.Unix(1000, 0)
	for _, kind := range []string{"tree", "rock", "campfire", "crystal", "tent"} {
		t.Run(kind, func(t *testing.T) {
			w := emptyPhysics(now)
			e := scene.Entity{ID: "solid", Kind: kind, X: 620, Y: 400, Radius: 22, Height: 120, Scale: 1}
			if kind == "tent" {
				e.Radius = 0
				e.HalfX = 22
				e.HalfY = 22
			}
			w.layout.Entities = []scene.Entity{e}
			for i := 1; i <= 20; i++ {
				r := w.Apply(&contracts.WorldReq{UserID: "a", SessionID: "a", Action: "input", DX: 1}, now.Add(time.Duration(i)*100*time.Millisecond))
				p := w.players["a"]
				if r.Error != "" || w.blocked(p, p.state.X, p.state.Y) {
					t.Fatal("entered solid")
				}
			}
			if w.players["a"].state.X > 578+epsilon {
				t.Fatal("tunneled through collider")
			}
			p := w.players["a"]
			// 被正面阻挡后仍能沿侧面移动，不把碰撞变成完全卡死。
			before := p.state.Y
			w.Apply(&contracts.WorldReq{UserID: "a", SessionID: "a", Action: "input", DX: 1, DY: 1}, now.Add(2100*time.Millisecond))
			if p.state.Y <= before || w.blocked(p, p.state.X, p.state.Y) {
				t.Fatal("no sliding")
			}
		})
	}
}
func TestLandOnSolidAndFallWhenWalkingOff(t *testing.T) {
	now := time.Unix(1000, 0)
	w := emptyPhysics(now)
	w.layout.Entities = []scene.Entity{{ID: "rock", Kind: "rock", X: 560, Y: 400, Radius: 25, Height: 20, Scale: 1}}
	p := w.players["a"]
	p.state.Z = 110
	p.state.Grounded = false
	r := command(w, "a", "a", "poll", now.Add(time.Second))
	if r.State.Players[0].Z != 20 || !p.state.Grounded {
		t.Fatal("not supported on top")
	}
	for i := 1; i <= 6; i++ {
		w.Apply(&contracts.WorldReq{UserID: "a", SessionID: "a", Action: "input", DX: 1}, now.Add(time.Second+time.Duration(i)*100*time.Millisecond))
	}
	command(w, "a", "a", "poll", now.Add(2*time.Second))
	if p.state.Z != 0 || !p.state.Grounded {
		t.Fatal("floating after leaving support")
	}
}
func TestPlayersCannotPassAndCanJumpOver(t *testing.T) {
	now := time.Unix(1000, 0)
	w := emptyPhysics(now)
	enter(w, "b", "b", now)
	// 单测设置可重复的对向起点；运行入口不存在改坐标接口。
	w.players["b"].state.X = 680
	for i := 1; i <= 10; i++ {
		w.Apply(&contracts.WorldReq{UserID: "a", SessionID: "a", Action: "input", DX: 1}, now.Add(time.Duration(i)*100*time.Millisecond))
	}
	p, b := w.players["a"], w.players["b"]
	if p.state.X > 640+epsilon || math.Hypot(p.state.X-b.state.X, p.state.Y-b.state.Y) < 40-epsilon {
		t.Fatal("players interpenetrated")
	}
	command(w, "a", "a", "jump", now.Add(time.Second))
	for i := 1; i <= 4; i++ {
		w.Apply(&contracts.WorldReq{UserID: "a", SessionID: "a", Action: "input", DX: 1}, now.Add(time.Second+time.Duration(i)*100*time.Millisecond))
	}
	if p.state.X <= 640 || p.state.Z <= playerHeight {
		t.Fatal("height did not allow jumping over player")
	}
	// 落在角色头顶必须是接触，而不是穿进身体。
	p.state.X = b.state.X
	p.state.Y = b.state.Y
	command(w, "a", "a", "poll", now.Add(2*time.Second))
	if p.state.Z != playerHeight || !p.state.Grounded {
		t.Fatal("no player support")
	}
}
func TestJumpHeadContactCeilingAndNoSafeSpawn(t *testing.T) {
	now := time.Unix(1000, 0)
	w := emptyPhysics(now)
	enter(w, "b", "b", now)
	w.players["b"].state.X = 560
	w.players["b"].state.Y = 400
	w.players["b"].state.Z = playerHeight
	command(w, "a", "a", "jump", now)
	command(w, "a", "a", "poll", now.Add(200*time.Millisecond))
	if w.players["a"].state.Z != 0 || w.blocked(w.players["a"], 560, 400) {
		t.Fatal("jump passed into overhead body")
	}
	w = emptyPhysics(now)
	p := w.players["a"]
	p.state.Z = ceiling - playerHeight - 1
	p.state.Grounded = false
	p.vz = jumpSpeed
	command(w, "a", "a", "poll", now.Add(100*time.Millisecond))
	if p.state.Z > ceiling-playerHeight {
		t.Fatal("world ceiling violated")
	}
	w.layout.Entities = []scene.Entity{{ID: "all", HalfX: 1200, HalfY: 800, X: 600, Y: 400, Height: 100}}
	if enter(w, "new", "new", now.Add(100*time.Millisecond)).Error != "world_full" {
		t.Fatal("unsafe spawn fallback")
	}
}

func TestMixedPhysicsNeverInterpenetrates(t *testing.T) {
	now := time.Unix(1000, 0)
	w := New()
	ids := []string{"a", "b", "c", "d"}
	for _, id := range ids {
		enter(w, id, id, now)
	}
	random := rand.New(rand.NewSource(17))
	for step := 1; step <= 1200; step++ {
		id := ids[step%len(ids)]
		req := &contracts.WorldReq{UserID: id, SessionID: id, Action: "input", DX: random.Float64()*2 - 1, DY: random.Float64()*2 - 1}
		if step%13 == 0 {
			req.Action = "jump"
		}
		w.Apply(req, now.Add(time.Duration(step)*10*time.Millisecond))
		for _, p := range w.players {
			if math.IsNaN(p.state.Z) || p.state.Z < 0 || p.state.Z > ceiling-playerHeight || w.blocked(p, p.state.X, p.state.Y) {
				t.Fatalf("body penetration at step %d: %+v", step, p.state)
			}
		}
	}
}

func TestJumpHeightAffectsGatherAndBounds(t *testing.T) {
	now := time.Unix(1000, 0)
	w := emptyPhysics(now)
	p := w.players["a"]
	p.state.Z = 90
	p.state.Grounded = false
	if command(w, "a", "a", "gather", now).Error != "no_resource_in_range" {
		t.Fatal("height ignored by gathering")
	}
	p.state.Z = 0
	p.state.Grounded = true
	if w.blocked(p, 19, 400) == false || w.blocked(p, 1181, 400) == false {
		t.Fatal("body crossed map edge")
	}
}
