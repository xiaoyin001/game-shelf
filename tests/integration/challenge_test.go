// 文件说明：真实四服务验证配对意图、隔离、严格输入和完整180秒回合，不缩短生产计时。
package integration

import (
	"game-shelf/pkg/contracts"
	"github.com/coder/websocket"
	"math"
	"testing"
	"time"
)

func walkWaypoint(t *testing.T, conn *websocket.Conn, id string, x, y float64) *contracts.WorldPlayer {
	t.Helper()
	var last *contracts.WorldPlayer
	for range 100 {
		p := findPlayer(t, read(t, conn, "state").State, id)
		last = p
		dx, dy := x-p.X, y-p.Y
		length := math.Hypot(dx, dy)
		if length < 18 {
			send(t, conn, "input", nil)
			return p
		}
		send(t, conn, "input", map[string]any{"dx": dx / length, "dy": dy / length})
	}
	t.Fatalf("cannot walk to (%v,%v), last %+v", x, y, last)
	return nil
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
	for _, field := range []string{"score", "duration", "end_ms", "combo", "owner_id"} {
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
			walkWaypoint(t, conn, account.UserID, i.X, i.Y)
			send(t, conn, "pick", map[string]any{"round_id": r.ID, "item_id": i.ID})
			challengeState(t, conn, func(s *contracts.Challenge) bool { return held(s, i.ID) })
		}
		walkWaypoint(t, conn, account.UserID, 700, 350)
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
