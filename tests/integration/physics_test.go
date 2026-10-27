// 文件说明：以真实四服务、双 WebSocket 和公开场景验证跳跃、实体阻挡与非法高度拒绝。
package integration

import (
	"encoding/json"
	"game-shelf/pkg/contracts"
	"game-shelf/pkg/scene"
	"math"
	"net/http"
	"testing"
)

func findPlayer(t *testing.T, state *contracts.WorldState, id string) *contracts.WorldPlayer {
	t.Helper()
	for _, p := range state.Players {
		if p.ID == id {
			return p
		}
	}
	t.Fatal("player missing from authoritative snapshot")
	return nil
}
func TestLiveJumpObservedByPeerWithoutInput(t *testing.T) {
	live(t)
	first, second := login(t, name()), login(t, name())
	a, b := auth(t, first), auth(t, second)
	send(t, a, "jump", nil)
	airborne, landed := false, false
	for range 18 {
		s := read(t, b, "state").State
		p := findPlayer(t, s, first.UserID)
		if s.SceneVersion != scene.Default().Version {
			t.Fatal("scene version mismatch")
		}
		if p.Z > 10 {
			airborne = true
		}
		if airborne && p.Grounded && p.Z == 0 {
			landed = true
			break
		}
	}
	if !airborne || !landed {
		t.Fatal("peer did not see real jump and landing without further input")
	}
}
func TestLiveSolidCrystalBlocksAndJumpPasses(t *testing.T) {
	live(t)
	reply := login(t, name())
	conn := auth(t, reply)
	initial := findPlayer(t, read(t, conn, "state").State, reply.UserID)
	// 选择出生点同排最近晶簇，始终只发方向，不传坐标或高度。
	target := scene.Default().Nodes[0]
	direction := 1.0
	if initial.X > target.X {
		direction = -1
	}
	for range 10 {
		send(t, conn, "input", map[string]any{"dx": direction})
		read(t, conn, "state")
	}
	blocked := findPlayer(t, read(t, conn, "state").State, reply.UserID)
	if d := math.Hypot(blocked.X-target.X, blocked.Y-target.Y); d < 42-1e-5 || d > 47 {
		t.Fatal("no solid crystal contact", d)
	}
	send(t, conn, "gather", nil)
	if read(t, conn, "error").Error != "unknown_action" {
		t.Fatal("retired gathering remained accessible")
	}
	for range 3 {
		send(t, conn, "input", map[string]any{"dx": direction})
		read(t, conn, "state")
	}
	cooling := findPlayer(t, read(t, conn, "state").State, reply.UserID)
	if math.Hypot(cooling.X-target.X, cooling.Y-target.Y) < 42-1e-5 {
		t.Fatal("decorative crystal lost its body")
	}
	send(t, conn, "jump", nil)
	crossed := false
	for range 12 {
		send(t, conn, "input", map[string]any{"dx": direction})
		p := findPlayer(t, read(t, conn, "state").State, reply.UserID)
		if (p.X-target.X)*direction > 42 {
			crossed = true
		}
	}
	if !crossed {
		t.Fatal("server jump did not permit passing low solid")
	}
}
func TestLiveCanonicalSceneAndForgedHeight(t *testing.T) {
	live(t)
	res, err := http.Get("http://127.0.0.1:11000/scene.json")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var layout scene.Layout
	if err := json.NewDecoder(res.Body).Decode(&layout); err != nil || layout.Version != scene.Default().Version || len(layout.Entities) != len(scene.Default().Entities) {
		t.Fatal("not serving canonical scene", err)
	}
	for _, field := range []string{"z", "grounded", "velocity_z"} {
		conn := auth(t, login(t, name()))
		send(t, conn, "input", map[string]any{field: 999})
		closed(t, conn)
	}
}
