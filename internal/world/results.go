// 文件说明：世界锁内冻结权威结算，后台领取后释放锁做I/O；只有真实保存确认才标记saved。
package world

import (
	"game-shelf/pkg/contracts"
	"github.com/golang/protobuf/proto"
	"sort"
	"time"
)

type resultJob struct {
	record *contracts.ScoreRecord
	status string
	next   time.Time
}

func (w *World) finalize(r *room) {
	if r.round == nil || r.qualified || r.round.Phase() != "finished" || r.eligibility() != "eligible" {
		return
	}
	s := r.round.Snapshot()
	if s.EndReason != "time_up" {
		return
	}
	r.qualified = true
	j := &resultJob{status: "pending", record: &contracts.ScoreRecord{RoundID: s.ID, Mode: s.Mode, Day: s.Day, Rules: s.Rules, StartMS: s.StartMS, EndMS: s.EndMS, Score: s.Score, Pairs: s.Pairs, Boards: s.Boards, Members: s.Members}}
	r.result = j
	w.results[s.ID] = j
}
func (r *room) settlement() *contracts.Settlement {
	if r.round == nil {
		return nil
	}
	status := "playing"
	if r.phase() == "finished" {
		switch {
		case r.result != nil:
			status = r.result.status
		case r.practice != "":
			status = "practice"
		case r.eligibility() == "reconnecting":
			status = "reconnecting"
		default:
			status = "abandoned"
		}
	}
	return &contracts.Settlement{RoundID: r.round.ID(), Status: status}
}

// ClaimResult 只允许每项一个在途提交；返回副本，调用者不能改变待存记录。
func (w *World) ClaimResult(now time.Time) *contracts.ScoreRecord {
	w.mu.Lock()
	defer w.mu.Unlock()
	ids := make([]string, 0, len(w.results))
	for id, j := range w.results {
		if j.status != "saving" && !now.Before(j.next) {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := w.results[ids[i]], w.results[ids[j]]
		if a.next.Equal(b.next) {
			return ids[i] < ids[j]
		}
		return a.next.Before(b.next)
	})
	j := w.results[ids[0]]
	j.status = "saving"
	return proto.Clone(j.record).(*contracts.ScoreRecord)
}

// CompleteResult 失败保留原记录并自动重试；离房/重开也不能取消待保存记录。
func (w *World) CompleteResult(id string, saved bool, now time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	j := w.results[id]
	if j == nil || j.status != "saving" {
		return
	}
	if saved {
		j.status = "saved"
		delete(w.results, id)
	} else {
		j.status = "failed"
		j.next = now.Add(5 * time.Second)
	}
}
