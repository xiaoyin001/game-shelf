// 文件说明：验证不可变成绩规范与计分边界，不用客户端自报成绩测试冒充真实完赛。
package scores

import (
	"fmt"
	"game-shelf/internal/challenge"
	"game-shelf/pkg/contracts"
	"github.com/golang/protobuf/proto"
	"testing"
	"time"
)

func record() *contracts.ScoreRecord {
	n := time.Date(2026, 10, 27, 23, 59, 59, 0, time.UTC)
	return &contracts.ScoreRecord{RoundID: fmt.Sprintf("%032x", 3), Mode: "duo", Day: "2026-10-27", Rules: challenge.Rules, StartMS: n.UnixMilli(), EndMS: n.Add(challenge.Duration).UnixMilli(), Score: 100, Pairs: 1, Members: []*contracts.ChallengeMember{{ID: fmt.Sprintf("%032x", 2), Name: "旅人乙", Pairs: 1}, {ID: fmt.Sprintf("%032x", 1), Name: "旅人甲"}}}
}
func TestCanonicalRecordAndValidation(t *testing.T) {
	r := record()
	n, party, err := Normalize(r)
	if err != nil || party != fmt.Sprintf("%032x:%032x", 1, 2) || n.Members[0].ID == r.Members[0].ID {
		t.Fatal("not canonical", err)
	}
	n.Members[0].Name = "不同"
	if r.Members[1].Name != "旅人甲" {
		t.Fatal("shared member")
	}
	for name, mutate := range map[string]func(*contracts.ScoreRecord){
		"id":             func(r *contracts.ScoreRecord) { r.RoundID = "bad" },
		"mode":           func(r *contracts.ScoreRecord) { r.Mode = "four" },
		"rules":          func(r *contracts.ScoreRecord) { r.Rules = "future" },
		"day":            func(r *contracts.ScoreRecord) { r.Day = "2026-10-28" },
		"duration":       func(r *contracts.ScoreRecord) { r.EndMS++ },
		"negative":       func(r *contracts.ScoreRecord) { r.Score = -1 },
		"inflated":       func(r *contracts.ScoreRecord) { r.Score = 150 },
		"boards":         func(r *contracts.ScoreRecord) { r.Boards = 1 },
		"contribution":   func(r *contracts.ScoreRecord) { r.Members[1].Pairs = 1 },
		"same account":   func(r *contracts.ScoreRecord) { r.Members[1].ID = r.Members[0].ID },
		"nil":            func(r *contracts.ScoreRecord) { r.Members[0] = nil },
		"capacity":       func(r *contracts.ScoreRecord) { r.Mode = "solo" },
		"malformed name": func(r *contracts.ScoreRecord) { r.Members[0].Name = " " },
	} {
		t.Run(name, func(t *testing.T) {
			v := proto.Clone(r).(*contracts.ScoreRecord)
			mutate(v)
			if _, _, err := Normalize(v); err == nil {
				t.Fatal("accepted invalid record")
			}
		})
	}
	zero := record()
	zero.Score = 0
	zero.Pairs = 0
	zero.Members[0].Pairs = 0
	if _, _, err := Normalize(zero); err != nil {
		t.Fatal("zero score is valid", err)
	}
	if !ValidQuery(r.Members[0].ID, "solo", "2026-10-27") || ValidQuery("bad", "solo", "2026-10-27") || ValidQuery(r.Members[0].ID, "duo", "2026-02-30") {
		t.Fatal("invalid query")
	}
}
