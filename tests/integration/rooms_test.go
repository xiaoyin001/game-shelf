// 文件说明：真实四服务验证双人共享、原成员重连、主动退出与完整三分钟。
package integration

import (
	"game-shelf/pkg/contracts"
	"github.com/coder/websocket"
	"math"
	"testing"
	"time"
)

func roomEvent(t *testing.T, c *websocket.Conn, valid func(event) bool) event {
	t.Helper()
	for range 50 {
		e := read(t, c, "state")
		if e.Room != nil && valid(e) {
			return e
		}
	}
	t.Fatal("room state did not converge")
	return event{}
}
func startCoop(t *testing.T, a, b *websocket.Conn) *contracts.ChallengeRoom {
	t.Helper()
	send(t, a, "create_room", nil)
	room := roomEvent(t, a, func(e event) bool { return e.Room.Phase == "waiting" }).Room
	send(t, b, "join_room", map[string]any{"room_code": room.Code})
	roomEvent(t, b, func(e event) bool { return len(e.Room.Members) == 2 })
	send(t, a, "start_duo", map[string]any{"room_id": room.ID})
	if read(t, a, "error").Error != "both_ready_required" {
		t.Fatal("started unprepared")
	}
	for _, c := range []*websocket.Conn{a, b} {
		send(t, c, "set_ready", map[string]any{"room_id": room.ID, "ready": true})
	}
	roomEvent(t, a, func(e event) bool {
		return len(e.Room.Members) == 2 && e.Room.Members[0].Ready && e.Room.Members[1].Ready
	})
	send(t, a, "start_duo", map[string]any{"room_id": room.ID})
	roomEvent(t, a, func(e event) bool { return e.Challenge != nil && e.Challenge.Phase == "running" })
	return room
}

// 两端持续消费快照，避免闲置连接的积压让移动意图突发越过限频。
func walkCoop(t *testing.T, actor, observer *websocket.Conn, id string, x, y float64) {
	t.Helper()
	for range 100 {
		p := findPlayer(t, read(t, actor, "state").State, id)
		read(t, observer, "state")
		dx, dy := x-p.X, y-p.Y
		length := math.Hypot(dx, dy)
		if length < 18 {
			send(t, actor, "input", nil)
			return
		}
		send(t, actor, "input", map[string]any{"dx": dx / length, "dy": dy / length})
	}
	t.Fatal("cooperative waypoint not reached", x, y)
}
func liveTeamPair(t *testing.T, a, b *websocket.Conn, aid, bid string, r *contracts.Challenge) {
	t.Helper()
	var first, mate *contracts.ChallengeItem
	for _, i := range r.Items {
		if i.Z > 0 && i.X == 740 && i.Y == 300 {
			first = i
			break
		}
	}
	for _, i := range r.Items {
		if i.ID != first.ID && i.Z > 0 && i.Kind == first.Kind {
			mate = i
			break
		}
	}
	walkCoop(t, a, b, aid, first.X, first.Y)
	send(t, a, "pick", map[string]any{"round_id": r.ID, "item_id": first.ID})
	challengeState(t, a, func(s *contracts.Challenge) bool { return held(s, first.ID) })
	walkCoop(t, a, b, aid, 700, 350)
	send(t, a, "put", map[string]any{"round_id": r.ID, "item_id": first.ID})
	challengeState(t, a, func(s *contracts.Challenge) bool {
		for _, i := range s.Items {
			if i.ID == first.ID && i.Place == "tray" {
				return true
			}
		}
		return false
	})
	walkCoop(t, a, b, aid, 780, 230)
	walkCoop(t, b, a, bid, mate.X, mate.Y)
	send(t, b, "pick", map[string]any{"round_id": r.ID, "item_id": mate.ID})
	challengeState(t, b, func(s *contracts.Challenge) bool { return held(s, mate.ID) })
	challengeState(t, a, func(s *contracts.Challenge) bool { return held(s, mate.ID) })
	walkCoop(t, b, a, bid, 700, 350)
	send(t, b, "put", map[string]any{"round_id": r.ID, "item_id": mate.ID})
	for _, c := range []*websocket.Conn{a, b} {
		challengeState(t, c, func(s *contracts.Challenge) bool { return s.Score == 100 && s.Pairs == 1 })
	}
}
func TestLiveCoopReconnectAndExplicitQuit(t *testing.T) {
	live(t)
	accountA, accountB := name(), name()
	la, lb := login(t, accountA), login(t, accountB)
	a, b := auth(t, la), auth(t, lb)
	room := startCoop(t, a, b)
	r := challengeState(t, a, func(s *contracts.Challenge) bool { return s.Mode == "duo" })
	third := auth(t, login(t, name()))
	send(t, third, "join_room", map[string]any{"room_code": room.Code})
	if read(t, third, "error").Error != "room_locked" {
		t.Fatal("mid-round substitution")
	}
	outsider := read(t, third, "state")
	if outsider.Room != nil || outsider.Challenge != nil || len(outsider.State.Players) != 1 {
		t.Fatal("room leaked")
	}
	liveTeamPair(t, a, b, la.UserID, lb.UserID, r)
	_ = b.CloseNow()
	roomEvent(t, a, func(e event) bool { return e.Room.Eligibility == "reconnecting" })
	b = auth(t, login(t, accountB))
	resumed := roomEvent(t, b, func(e event) bool { return e.Room.Eligibility == "eligible" })
	if resumed.Challenge.ID != r.ID || resumed.Challenge.EndMS != r.EndMS || resumed.Challenge.Score != 100 {
		t.Fatal("reconnect reset shared round")
	}
	send(t, b, "leave_world", map[string]any{"room_id": room.ID})
	read(t, b, "left")
	closed(t, b)
	practice := roomEvent(t, a, func(e event) bool { return e.Room.Eligibility == "practice" })
	if practice.Room.PracticeReason != "member_left" || practice.Challenge.EndMS != r.EndMS {
		t.Fatal("quit did not practice")
	}
	send(t, third, "join_room", map[string]any{"room_code": room.Code})
	if read(t, third, "error").Error != "room_locked" {
		t.Fatal("practice accepted replacement")
	}
}
func TestLiveCoopComplete180Seconds(t *testing.T) {
	live(t)
	t.Parallel()
	la, lb := login(t, name()), login(t, name())
	a, b := auth(t, la), auth(t, lb)
	started := time.Now()
	room := startCoop(t, a, b)
	r := challengeState(t, a, func(s *contracts.Challenge) bool { return s.Mode == "duo" })
	liveTeamPair(t, a, b, la.UserID, lb.UserID, r)
	nextPing := time.Now()
	var ended event
	for time.Since(started) < 195*time.Second {
		ea, eb := read(t, a, "state"), read(t, b, "state")
		if time.Now().After(nextPing) {
			send(t, a, "ping", nil)
			send(t, b, "ping", nil)
			nextPing = time.Now().Add(time.Second)
		}
		if ea.Challenge != nil && eb.Challenge != nil && ea.Challenge.Phase == "finished" && eb.Challenge.Phase == "finished" {
			ended = ea
			break
		}
	}
	if ended.Challenge == nil || ended.Challenge.Score != 100 || ended.Challenge.Pairs != 1 || ended.Challenge.EndMS-r.StartMS != 180000 || ended.Room.Eligibility != "eligible" || time.Since(started) < 180*time.Second {
		t.Fatal("not complete authoritative coop round", ended)
	}
	waitSaved(t, a, r.ID)
	waitSaved(t, b, r.ID)
	board := queryScores(t, b, "duo", r.Day)
	if board.BestDuo == nil || board.BestDuo.RoundID != r.ID || board.BestDuo.Score != 100 || len(board.BestDuo.Members) != 2 {
		t.Fatal("duo not persisted", board)
	}
	send(t, a, "rematch", map[string]any{"room_id": room.ID})
	waiting := roomEvent(t, a, func(e event) bool { return e.Room.Phase == "waiting" })
	if waiting.Challenge != nil || waiting.Room.Members[0].Ready || waiting.Room.Members[1].Ready {
		t.Fatal("rematch skipped preparation")
	}
	t.Logf("real coop elapsed=%s score=%d members=%d; both return to preparation", time.Since(started), ended.Challenge.Score, len(ended.Challenge.Members))
}
