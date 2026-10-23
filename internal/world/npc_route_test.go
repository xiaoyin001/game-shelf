// 文件说明：复现委托返程的晶簇北缘位置；原直线必须受阻，真实跳跃可以越过且不穿透。
package world

import (
	"game-shelf/pkg/contracts"
	"testing"
	"time"
)

func TestQuestReturnAtCrystalNorthEdge(t *testing.T) {
	now := time.Unix(1000, 0)
	w := New()
	enter(w, "a", "a", now)
	p := w.players["a"]
	// 仅单测复现失败现场；客户端没有设置坐标的接口。
	p.state.X, p.state.Y = 874, 564
	if w.blocked(p, p.state.X, p.state.Y) {
		t.Fatal("fixture already inside collider")
	}
	for i := 1; i <= 10; i++ {
		r := w.Apply(&contracts.WorldReq{UserID: "a", SessionID: "a", Action: "input", DX: -1}, now.Add(time.Duration(i)*100*time.Millisecond))
		if r.Error != "" || w.blocked(p, p.state.X, p.state.Y) {
			t.Fatal("west movement penetrated crystal", r.Error, p.state)
		}
	}
	if p.state.X < 870 || p.state.X > 874 || !p.state.Grounded {
		t.Fatal("original west route was not obstructed", p.state)
	}
	if r := command(w, "a", "a", "jump", now.Add(time.Second)); r.Error != "" {
		t.Fatal("grounded jump failed", r.Error)
	}
	peak := p.state.Z
	for i := 1; i <= 15; i++ {
		r := w.Apply(&contracts.WorldReq{UserID: "a", SessionID: "a", Action: "input", DX: -1}, now.Add(time.Second+time.Duration(i)*100*time.Millisecond))
		if r.Error != "" || w.blocked(p, p.state.X, p.state.Y) {
			t.Fatal("jump return penetrated collider", r.Error, p.state)
		}
		if p.state.Z > peak {
			peak = p.state.Z
		}
	}
	// 原场景散石仍可被迈上；停步后按真实重力落地，不把半途下降当成悬空。
	command(w, "a", "a", "poll", now.Add(3*time.Second))
	if peak <= 50 || p.state.X >= 808 || p.state.Z != 0 || !p.state.Grounded {
		t.Fatal("jump did not clear crystal and land", peak, p.state)
	}
}
