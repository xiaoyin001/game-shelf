// 文件说明：验证准确截止资格冻结、独立待存队列和失败重试，不在生产提供缩时或填分入口。
package world

import (
	"game-shelf/pkg/contracts"
	"testing"
	"time"
)

func TestResultFrozenAtDeadlineSurvivesLeaveAndRetry(t *testing.T) {
	w, n, r := duo(t)
	for i := 1; i < 180; i++ {
		for _, id := range []string{"a", "b"} {
			command(w, id, id, "poll", n.Add(time.Duration(i)*time.Second))
		}
	}
	if w.ClaimResult(n.Add(179*time.Second)) != nil {
		t.Fatal("early result")
	}
	end := n.Add(180 * time.Second)
	s := command(w, "a", "a", "poll", end)
	if s.Settlement == nil || s.Settlement.Status != "pending" {
		t.Fatal("no pending result", s)
	}
	job := w.ClaimResult(end)
	if job == nil || job.RoundID != r.Challenge.ID || job.Score != 0 || len(job.Members) != 2 || w.ClaimResult(end) != nil {
		t.Fatal("duplicate/missing result")
	}
	job.Members[0].Name = "外部修改"
	w.CompleteResult(job.RoundID, false, end)
	if command(w, "a", "a", "poll", end).Settlement.Status != "failed" || w.ClaimResult(end.Add(4999*time.Millisecond)) != nil {
		t.Fatal("retry not bounded")
	}
	// 已确认完赛资格后离房不改成绩，待存生命周期不依赖房间。
	roomReq(w, "b", "leave_round", r.Room.ID, end, false)
	if command(w, "a", "a", "poll", end).Room.Eligibility != "eligible" {
		t.Fatal("qualified finish invalidated")
	}
	roomReq(w, "a", "rematch", r.Room.ID, end, false)
	retry := w.ClaimResult(end.Add(5 * time.Second))
	if retry == nil || retry.Members[0].Name == "外部修改" {
		t.Fatal("record mutable or lost by rematch")
	}
	w.CompleteResult(retry.RoundID, true, end)
	if len(w.results) != 0 || w.ClaimResult(end.Add(9*time.Second)) != nil {
		t.Fatal("saved result not consumed")
	}
}
func TestPracticeAndAbandonNeverSubmit(t *testing.T) {
	w, n, r := duo(t)
	roomReq(w, "b", "leave_round", r.Room.ID, n, false)
	for i := 1; i <= 180; i++ {
		command(w, "a", "a", "poll", n.Add(time.Duration(i)*time.Second))
	}
	if w.ClaimResult(n.Add(180*time.Second)) != nil || command(w, "a", "a", "poll", n.Add(180*time.Second)).Settlement.Status != "practice" {
		t.Fatal("practice submitted")
	}
	w = New()
	enter(w, "a", "a", n)
	command(w, "a", "a", "start_solo", n)
	command(w, "a", "a", "leave", n)
	w.Tick(n.Add(180 * time.Second))
	if len(w.results) != 0 {
		t.Fatal("abandoned solo submitted")
	}
}
func TestResultWaitsForGraceAndSubmitsOnceOnRestore(t *testing.T) {
	w, n, r := duo(t)
	for i := 1; i <= 175; i++ {
		for _, id := range []string{"a", "b"} {
			command(w, id, id, "poll", n.Add(time.Duration(i)*time.Second))
		}
	}
	command(w, "b", "b", "leave", n.Add(175*time.Second))
	for i := 176; i <= 190; i++ {
		command(w, "a", "a", "poll", n.Add(time.Duration(i)*time.Second))
	}
	if w.ClaimResult(n.Add(190*time.Second)) != nil {
		t.Fatal("offline finish submitted before confirmation")
	}
	fresh := enter(w, "b", "new", n.Add(190*time.Second))
	if fresh.Challenge.ID != r.Challenge.ID || fresh.Settlement.Status != "pending" {
		t.Fatal("restore did not settle")
	}
	if w.ClaimResult(n.Add(190*time.Second)) == nil || w.ClaimResult(n.Add(190*time.Second)) != nil {
		t.Fatal("not once")
	}
}
func TestFinishedSoloSurvivesExplicitLeaveAndQueueBackpressure(t *testing.T) {
	w := New()
	n := time.Unix(1000, 0)
	enter(w, "a", "a", n)
	r := command(w, "a", "a", "start_solo", n)
	for i := 1; i <= 180; i++ {
		command(w, "a", "a", "poll", n.Add(time.Duration(i)*time.Second))
	}
	s := command(w, "a", "a", "leave_world", n.Add(180*time.Second))
	if s.Error != "" || w.ClaimResult(n.Add(180*time.Second)).RoundID != r.Challenge.ID {
		t.Fatal("leave lost solo")
	}
	w = New()
	enter(w, "a", "a", n)
	for i := 0; i < 100; i++ {
		w.results[time.Unix(int64(i), 0).String()] = &resultJob{status: "saving", record: &contracts.ScoreRecord{}}
	}
	if command(w, "a", "a", "start_solo", n).Error != "results_busy" {
		t.Fatal("unbounded admission")
	}
}
func TestScoresQueryRequiresCurrentSessionAndRateLimit(t *testing.T) {
	w := New()
	n := time.Unix(1000, 0)
	enter(w, "a", "s", n)
	if command(w, "a", "wrong", "leaderboard", n).Error != "session_replaced_or_expired" {
		t.Fatal("unbound query")
	}
	if command(w, "a", "s", "leaderboard", n).Error != "" || command(w, "a", "s", "leaderboard", n.Add(799*time.Millisecond)).Error != "board_too_fast" || command(w, "a", "s", "leaderboard", n.Add(800*time.Millisecond)).Error != "" {
		t.Fatal("query limit")
	}
}
