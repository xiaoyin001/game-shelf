// 文件说明：真实四服务验证配对意图、隔离、严格输入和完整180秒回合，不缩短生产计时。
package integration

import (
	"game-shelf/pkg/contracts"
	"game-shelf/pkg/scene"
	"game-shelf/tests/navigation"
	"github.com/coder/websocket"
	"math"
	"testing"
	"time"
)

func walkWaypoint(t *testing.T, conn *websocket.Conn, id string, x, y float64) *contracts.WorldPlayer {
	t.Helper()
	p := findPlayer(t, freshWorld(t, conn, time.Now().UnixMilli()), id)
	for range 100 {
		dx, dy := x-p.X, y-p.Y
		length := math.Hypot(dx, dy)
		if length < 18 {
			send(t, conn, "input", nil)
			return p
		}
		marker := time.Now().UnixMilli()
		send(t, conn, "input", map[string]any{"dx": dx / length, "dy": dy / length})
		p = findPlayer(t, freshWorld(t, conn, marker), id)
	}
	t.Fatal("physical waypoint not reached", p, x, y)
	return nil
}

// freshWorld 持续消费真实旧快照，不能用积压坐标连续突发补发移动。
func freshWorld(t *testing.T, c *websocket.Conn, since int64) *contracts.WorldState {
	t.Helper()
	for range 300 {
		s := read(t, c, "state").State
		if s.TimeMS >= since {
			return s
		}
	}
	t.Fatal("fresh authoritative snapshot missing")
	return nil
}

// walkGround 按公开真实布局绕障，所有位置都由真实WebSocket方向输入推进。
func walkGround(t *testing.T, conn, observer *websocket.Conn, id string, x, y float64) *contracts.WorldPlayer {
	t.Helper()
	marker := time.Now().UnixMilli()
	state := freshWorld(t, conn, marker)
	if observer != nil {
		freshWorld(t, observer, marker)
	}
	p := findPlayer(t, state, id)
	bodies := []scene.Point{}
	for _, b := range state.Players {
		if b.ID != id {
			bodies = append(bodies, scene.Point{X: b.X, Y: b.Y})
		}
	}
	path := navigation.Route(scene.Default(), scene.Point{X: p.X, Y: p.Y}, scene.Point{X: x, Y: y}, bodies)
	if len(path) == 0 {
		t.Fatal("no public ground route", p, x, y)
	}
	for _, next := range path {
		reached := false
		for range 100 {
			dx, dy := next.X-p.X, next.Y-p.Y
			length := math.Hypot(dx, dy)
			if length < 3 {
				reached = true
				break
			}
			scale := math.Max(18, length)
			marker = time.Now().UnixMilli()
			send(t, conn, "input", map[string]any{"dx": dx / scale, "dy": dy / scale})
			p = findPlayer(t, freshWorld(t, conn, marker), id)
			if observer != nil {
				freshWorld(t, observer, marker)
			}
		}
		if !reached {
			t.Fatal("real ground route stuck", p, next, x, y)
		}
	}
	send(t, conn, "input", nil)
	return p
}
func challengeState(t *testing.T, c *websocket.Conn, valid func(*contracts.Challenge) bool) *contracts.Challenge {
	t.Helper()
	for range 50 {
		s := read(t, c, "state").Challenge
		if s != nil && valid(s) {
			return s
		}
	}
	t.Fatal("challenge snapshot did not converge")
	return nil
}
func held(s *contracts.Challenge, id string) bool {
	for _, i := range s.Items {
		if i.ID == id {
			return i.Place == "hand"
		}
	}
	return false
}
func TestLiveChallengeIsolationAndUntrustedScore(t *testing.T) {
	live(t)
	a, b := auth(t, login(t, name())), auth(t, login(t, name()))
	send(t, a, "start_solo", nil)
	r := challengeState(t, a, func(s *contracts.Challenge) bool { return s.Phase == "running" })
	if r.EndMS-r.StartMS != 180000 || r.Score != 0 || len(r.Items) != 16 {
		t.Fatal("invalid initial round", r)
	}
	if s := read(t, b, "state"); s.Challenge != nil || len(s.State.Players) != 1 {
		t.Fatal("private round leaked")
	}
	item := r.Items[0]
	send(t, a, "pick", map[string]any{"round_id": r.ID, "item_id": item.ID})
	if read(t, a, "error").Error != "item_out_of_range" {
		t.Fatal("remote pick accepted")
	}
	for _, field := range []string{"score", "duration", "end_ms", "combo", "owner_id", "wading", "speed"} {
		conn := auth(t, login(t, name()))
		send(t, conn, "put", map[string]any{field: 999})
		closed(t, conn)
	}
	send(t, a, "gather", nil)
	if read(t, a, "error").Error != "unknown_action" {
		t.Fatal("old score entry active")
	}
	send(t, a, "leave_round", nil)
}
func TestLiveChallengeComplete180Seconds(t *testing.T) {
	live(t)
	t.Parallel()
	accountName := name()
	account := login(t, accountName)
	conn := auth(t, account)
	started := time.Now()
	send(t, conn, "start_solo", nil)
	r := challengeState(t, conn, func(s *contracts.Challenge) bool { return s.Phase == "running" })
	privateRoundReady(t)
	firstSnapshotDelay := time.Since(started)
	current := r
	for pair := int32(1); pair <= 8; pair++ {
		var first, second *contracts.ChallengeItem
		for _, i := range current.Items {
			if i.Place != "ground" || i.Blocked {
				continue
			}
			for _, j := range current.Items {
				if i.ID != j.ID && j.Place == "ground" && !j.Blocked && i.Kind == j.Kind {
					first, second = i, j
					break
				}
			}
			if first != nil {
				break
			}
		}
		if first == nil {
			t.Fatal("no open pair in real board")
		}
		for _, i := range []*contracts.ChallengeItem{first, second} {
			walkGround(t, conn, nil, account.UserID, i.X, i.Y)
			send(t, conn, "pick", map[string]any{"round_id": r.ID, "item_id": i.ID})
			challengeState(t, conn, func(s *contracts.Challenge) bool { return held(s, i.ID) })
		}
		walkGround(t, conn, nil, account.UserID, 700, 350)
		for _, i := range []*contracts.ChallengeItem{first, second} {
			send(t, conn, "put", map[string]any{"round_id": r.ID, "item_id": i.ID})
			read(t, conn, "state")
		}
		current = challengeState(t, conn, func(s *contracts.Challenge) bool { return s.Pairs == pair })
	}
	if current.Boards != 1 || current.Board != 2 || len(current.Items) != 16 || current.Score < 1300 {
		t.Fatal("real board not cleared/replenished", current)
	}
	finalScore := current.Score
	nextPing := time.Now()
	var finished *contracts.Challenge
	for time.Since(started) < 190*time.Second {
		s := read(t, conn, "state").Challenge
		if time.Now().After(nextPing) {
			send(t, conn, "ping", nil)
			nextPing = time.Now().Add(time.Second)
		}
		if s != nil && s.Phase == "finished" {
			finished = s
			break
		}
	}
	if finished == nil || finished.Score != finalScore || finished.Pairs != 8 || finished.Boards != 1 || finished.EndMS != r.EndMS || finished.EndReason != "time_up" || finished.RemainingMS != 0 || time.Since(started) < 180*time.Second {
		t.Fatal("not a real complete 180-second round", finished, time.Since(started))
	}
	send(t, conn, "pick", map[string]any{"round_id": r.ID, "item_id": r.Items[1].ID})
	if read(t, conn, "error").Error != "round_finished" {
		t.Fatal("late pickup accepted")
	}
	waitSaved(t, conn, r.ID)
	board := queryScores(t, conn, "solo", r.Day)
	if board.BestSolo == nil || board.BestSolo.RoundID != r.ID || board.BestSolo.Score != finalScore {
		t.Fatal("finished solo not persisted", board)
	}
	send(t, conn, "start_solo", nil)
	fresh := challengeState(t, conn, func(s *contracts.Challenge) bool { return s.ID != r.ID })
	if fresh.Score != 0 || fresh.EndMS-fresh.StartMS != 180000 {
		t.Fatal("rematch did not reset")
	}
	conn = auth(t, login(t, accountName))
	board = queryScores(t, conn, "solo", r.Day)
	if board.BestSolo == nil || board.BestSolo.RoundID != r.ID {
		t.Fatal("password relogin lost best", board)
	}
	t.Logf("real fixed round: elapsed=%s first_snapshot_delay=%s score=%d end_ms=%d; rematch=%s", time.Since(started), firstSnapshotDelay, finished.Score, finished.EndMS, fresh.ID)
}
