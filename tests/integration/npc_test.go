// 文件说明：真实双连接完成 NPC 委托，验证私有对话、公共奖励与恶意请求拒绝。
package integration

import (
	"game-shelf/pkg/contracts"
	"github.com/coder/websocket"
	"math"
	"testing"
)

func walkNPC(t *testing.T, conn *websocket.Conn, id string, x, y float64) *contracts.WorldPlayer {
	t.Helper()
	var last *contracts.WorldPlayer
	for range 80 {
		p := findPlayer(t, read(t, conn, "state").State, id)
		last = p
		dx, dy := x-p.X, y-p.Y
		if math.Hypot(dx, dy) < 18 {
			return p
		}
		length := math.Hypot(dx, dy)
		send(t, conn, "input", map[string]any{"dx": dx / length, "dy": dy / length})
	}
	t.Fatalf("could not walk to NPC waypoint (%v, %v), last player: %+v", x, y, last)
	return nil
}
func TestLiveNPCQuestAndPrivacy(t *testing.T) {
	live(t)
	first, second := login(t, name()), login(t, name())
	a, b := auth(t, first), auth(t, second)
	send(t, a, "interact", map[string]any{"npc_id": "keeper"})
	if read(t, a, "error").Error != "npc_out_of_range" {
		t.Fatal("remote interaction accepted")
	}
	walkNPC(t, a, first.UserID, 560, 448)
	send(t, a, "interact", map[string]any{"npc_id": "keeper"})
	if r := read(t, a, "dialog"); r.Dialog.NPCID != "keeper" || len(r.Dialog.Choices) == 0 {
		t.Fatal(r)
	}
	send(t, a, "choose", map[string]any{"npc_id": "keeper", "choice_id": "complete"})
	if read(t, a, "error").Error != "quest_not_ready" {
		t.Fatal("unearned quest accepted")
	}
	send(t, a, "choose", map[string]any{"npc_id": "keeper", "choice_id": "accept"})
	if r := read(t, a, "dialog"); r.Quest.Status != "active" {
		t.Fatal(r)
	}
	walkNPC(t, a, first.UserID, 560, 400)
	walkNPC(t, a, first.UserID, 578, 400)
	for i := 1; i <= 2; i++ {
		send(t, a, "gather", nil)
		var last event
		for range 10 {
			last = read(t, a, "state")
			if last.Quest.Progress == int32(i) {
				break
			}
		}
		if last.Quest.Progress != int32(i) {
			t.Fatal("gather not counted", last)
		}
		if i == 1 {
			ready := last.State.Nodes[0].ReadyAtMS
			for last.State.TimeMS < ready {
				last = read(t, a, "state")
			}
		}
	}
	walkNPC(t, a, first.UserID, 560, 448)
	for range 2 {
		send(t, a, "choose", map[string]any{"npc_id": "keeper", "choice_id": "complete"})
		r := read(t, a, "dialog")
		if r.Quest.Status != "completed" || findPlayer(t, r.State, first.UserID).Score != 5 {
			t.Fatal("reward not unique", r)
		}
	}
	observed := false
	for range 200 {
		r := read(t, b, "state")
		if r.Dialog != nil || r.Quest == nil || r.Quest.Status != "" || r.Quest.Progress != 0 {
			t.Fatal("private NPC state leaked", r)
		}
		if findPlayer(t, r.State, first.UserID).Score == 5 {
			observed = true
			break
		}
	}
	if !observed {
		t.Fatal("peer did not observe quest reward")
	}
	// 与后续既有采集回归共享真实世界，等自然重生，不重置或篡改服务端资源。
	for {
		r := read(t, a, "state")
		if r.State.TimeMS >= r.State.Nodes[0].ReadyAtMS {
			break
		}
	}
}
func TestLiveNPCSolidAndRangerOptions(t *testing.T) {
	live(t)
	reply := login(t, name())
	conn := auth(t, reply)
	walkNPC(t, conn, reply.UserID, 560, 300)
	walkNPC(t, conn, reply.UserID, 700, 360)
	for range 8 {
		send(t, conn, "input", map[string]any{"dy": -1})
		read(t, conn, "state")
	}
	p := findPlayer(t, read(t, conn, "state").State, reply.UserID)
	if distance := math.Hypot(p.X-700, p.Y-230); distance < 39.99 || distance > 43 {
		t.Fatal("NPC cylinder did not block movement", p)
	}
	for _, choice := range []string{"camp", "crystals"} {
		send(t, conn, "choose", map[string]any{"npc_id": "ranger", "choice_id": choice})
		r := read(t, conn, "dialog")
		if r.Dialog.NPCID != "ranger" || r.Dialog.Text == "" {
			t.Fatal("missing ranger dialogue", r)
		}
	}
}
func TestLiveNPCRejectsForgedProgress(t *testing.T) {
	live(t)
	for _, field := range []string{"reward", "progress", "quest", "score"} {
		conn := auth(t, login(t, name()))
		send(t, conn, "choose", map[string]any{"npc_id": "keeper", "choice_id": "complete", field: 999})
		closed(t, conn)
	}
	// 仍以正常业务错误拒绝未知选项，不让传入字符串变成任务脚本。
	conn := auth(t, login(t, name()))
	send(t, conn, "interact", map[string]any{"npc_id": "<script>"})
	if read(t, conn, "error").Error != "unknown_npc" {
		t.Fatal("unknown NPC accepted")
	}
}
