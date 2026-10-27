// 文件说明：真实Mongo验证不可变成绩与查询；公开服务的完整180秒成绩另由实际回合产生。
package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"game-shelf/internal/challenge"
	"game-shelf/internal/scores"
	"game-shelf/pkg/contracts"
	"github.com/coder/websocket"
	"github.com/golang/protobuf/proto"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"sync"
	"testing"
	"time"
)

func scoreID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func scoreSample(mode string, start time.Time, points, pairs int32, ids ...string) *contracts.ScoreRecord {
	members := make([]*contracts.ChallengeMember, 0, len(ids))
	for i, id := range ids {
		m := &contracts.ChallengeMember{ID: id, Name: "存储旅人"}
		if i == 0 {
			m.Pairs = pairs
		}
		members = append(members, m)
	}
	return &contracts.ScoreRecord{RoundID: scoreID(), Mode: mode, Day: start.UTC().Format("2006-01-02"), Rules: challenge.Rules, StartMS: start.UnixMilli(), EndMS: start.Add(challenge.Duration).UnixMilli(), Score: points, Pairs: pairs, Boards: pairs / 8, Members: members}
}
func TestLiveScoreStoreIdempotenceDailyPartiesAndBest(t *testing.T) {
	live(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:27017"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(ctx)
	// 仅创建与回收本用例随机新库，不触碰game_shelf或既有档案。
	db := client.Database("score_test_" + scoreID())
	defer db.Drop(ctx)
	store, err := scores.NewStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	a, b, c := scoreID(), scoreID(), scoreID()
	n := time.Date(2026, 1, 17, 12, 0, 0, 0, time.UTC)
	low := scoreSample("solo", n, 100, 1, a)
	high := scoreSample("solo", n.Add(20*time.Second), 250, 2, a)
	earlier := scoreSample("solo", n.Add(10*time.Second), 250, 2, b)
	ab := scoreSample("duo", n, 100, 1, a, b)
	ba := scoreSample("duo", n.Add(time.Second), 250, 2, b, a)
	ac := scoreSample("duo", n, 450, 3, a, c)
	tomorrow := scoreSample("solo", n.Add(24*time.Hour), 450, 3, a)
	for _, r := range []*contracts.ScoreRecord{low, high, earlier, ab, ba, ac, tomorrow} {
		if err = store.Save(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for range 12 {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- store.Save(ctx, high) }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if count, e := db.Collection("challenge_results").CountDocuments(ctx, bson.M{"_id": high.RoundID}); e != nil || count != 1 {
		t.Fatal("not idempotent", count, e)
	}
	conflict := proto.Clone(low).(*contracts.ScoreRecord)
	conflict.Score = 250
	conflict.Pairs = 2
	conflict.Members[0].Pairs = 2
	if !errors.Is(store.Save(ctx, conflict), scores.ErrConflict) {
		t.Fatal("conflicting record overwritten")
	}
	board, err := store.Query(ctx, a, "solo", "2026-01-17")
	if err != nil || len(board.Entries) != 2 || board.Entries[0].RoundID != earlier.RoundID || board.Entries[1].RoundID != high.RoundID || board.BestSolo.RoundID != tomorrow.RoundID || board.BestDuo.RoundID != ac.RoundID {
		t.Fatal("daily/best/tie not independent", board, err)
	}
	board, err = store.Query(ctx, a, "duo", "2026-01-17")
	if err != nil || len(board.Entries) != 2 || board.Entries[0].RoundID != ac.RoundID || board.Entries[1].RoundID != ba.RoundID {
		t.Fatal("unordered party did not keep highest", board, err)
	}
	// 新句柄/索引初始化仍读同一已确认记录；不是伪称已重启四服务。
	restored, err := scores.NewStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	board, err = restored.Query(ctx, c, "solo", "2026-01-18")
	if err != nil || len(board.Entries) != 1 || board.BestSolo != nil || board.BestDuo.RoundID != ac.RoundID {
		t.Fatal("restore/privacy", board, err)
	}
	unknown := scoreSample("solo", n, 100, 1, a)
	raw, _ := bson.Marshal(unknown)
	var doc bson.M
	_ = bson.Unmarshal(raw, &doc)
	doc["_id"] = unknown.RoundID
	doc["schema"] = 99
	doc["party"] = a
	if _, err = db.Collection("challenge_results").InsertOne(ctx, doc); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(restored.Save(ctx, unknown), scores.ErrConflict) {
		t.Fatal("unknown schema overwritten")
	}
	doc["_id"] = scoreID()
	doc["schema"] = 1
	doc["rules"] = "pair-future"
	if _, err = db.Collection("challenge_results").InsertOne(ctx, doc); err != nil {
		t.Fatal(err)
	}
	board, err = restored.Query(ctx, a, "solo", "2026-01-17")
	if err != nil || len(board.Entries) != 2 {
		t.Fatal("cross-rule leak", err)
	}
	// 前20有界且按实际完成时间稳定排序，0分正常完赛也能保存。
	for i := 0; i < 25; i++ {
		r := scoreSample("solo", n.Add(48*time.Hour+time.Duration(i)*time.Second), 100, 1, scoreID())
		if err = restored.Save(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	board, err = restored.Query(ctx, a, "solo", "2026-01-19")
	if err != nil || len(board.Entries) != 20 {
		t.Fatal("top20 not bounded", err)
	}
	for i := 1; i < len(board.Entries); i++ {
		if board.Entries[i-1].EndMS >= board.Entries[i].EndMS {
			t.Fatal("tie ordering unstable")
		}
	}
	zero := scoreSample("solo", n.Add(72*time.Hour), 0, 0, c)
	if err = restored.Save(ctx, zero); err != nil {
		t.Fatal(err)
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	if err = restored.Save(cancelled, scoreSample("solo", n, 100, 1, a)); err == nil {
		t.Fatal("cancelled save reported success")
	}
	if _, err = restored.Query(ctx, a, "solo", "2026-02-30"); err == nil {
		t.Fatal("bad day accepted")
	}
	// 当前真实collection无此用例写入，低层夹具不冒充公开对局。
}
func waitSaved(t *testing.T, c *websocket.Conn, id string) {
	t.Helper()
	for range 50 {
		e := read(t, c, "state")
		if e.Settlement != nil && e.Settlement.RoundID == id && e.Settlement.Status == "saved" {
			return
		}
	}
	t.Fatal("authoritative result was not acknowledged by storage", id)
}
func queryScores(t *testing.T, c *websocket.Conn, mode, day string) *contracts.Leaderboard {
	t.Helper()
	send(t, c, "leaderboard", map[string]any{"mode": mode, "day": day})
	for range 50 {
		e := read(t, c, "state")
		if e.Board != nil {
			if e.Board.Error != "" {
				t.Fatal(e.Board.Error)
			}
			return e.Board
		}
	}
	t.Fatal("no real leaderboard response")
	return nil
}
func TestLivePracticeDoesNotPersist180Seconds(t *testing.T) {
	live(t)
	t.Parallel()
	la, lb := login(t, name()), login(t, name())
	a, b := auth(t, la), auth(t, lb)
	started := time.Now()
	room := startCoop(t, a, b)
	r := challengeState(t, a, func(s *contracts.Challenge) bool { return s.Mode == "duo" })
	liveTeamPair(t, a, b, la.UserID, lb.UserID, r)
	send(t, b, "leave_world", map[string]any{"room_id": room.ID})
	read(t, b, "left")
	nextPing := time.Now()
	var end event
	for time.Since(started) < 195*time.Second {
		e := read(t, a, "state")
		if time.Now().After(nextPing) {
			send(t, a, "ping", nil)
			nextPing = time.Now().Add(time.Second)
		}
		if e.Challenge != nil && e.Challenge.Phase == "finished" {
			end = e
			break
		}
	}
	if end.Challenge == nil || end.Challenge.Score != 100 || end.Room.Eligibility != "practice" || end.Settlement.Status != "practice" || time.Since(started) < 180*time.Second {
		t.Fatal("not real completed practice", end)
	}
	board := queryScores(t, a, "duo", r.Day)
	for _, row := range board.Entries {
		if row.RoundID == r.ID {
			t.Fatal("practice ranked")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:27017"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(ctx)
	if count, e := client.Database("game_shelf").Collection("challenge_results").CountDocuments(ctx, bson.M{"_id": r.ID}); e != nil || count != 0 {
		t.Fatal("practice persisted", count, e)
	}
	t.Logf("real practice elapsed=%s score=100 not stored", time.Since(started))
}
