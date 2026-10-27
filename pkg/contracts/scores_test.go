// 文件说明：验证新增成绩/榜单/保存状态字段完整protobuf往返，旧字段号不复用。
package contracts

import (
	"github.com/golang/protobuf/proto"
	"testing"
)

func TestScoresWireRoundTrip(t *testing.T) {
	r := &ScoreRecord{RoundID: "round", Mode: "duo", Day: "2026-10-27", Rules: "pair-1", StartMS: 1, EndMS: 180001, Score: 100, Pairs: 1, Members: []*ChallengeMember{{ID: "a", Name: "甲"}, {ID: "b", Name: "乙", Pairs: 1}}}
	messages := []proto.Message{&ScoresRequest{UserID: "a", Mode: "solo", Day: r.Day, Record: r}, &ScoresReply{Board: &Leaderboard{Mode: "duo", Day: r.Day, Rules: r.Rules, Entries: []*ScoreRecord{r}, BestDuo: r}}, &WorldResp{Settlement: &Settlement{RoundID: r.RoundID, Status: "saved"}, Board: &Leaderboard{Entries: []*ScoreRecord{r}}}, &WorldReq{Action: "leaderboard", Mode: "duo", Day: r.Day}}
	for _, m := range messages {
		b, e := proto.Marshal(m)
		if e != nil {
			t.Fatal(e)
		}
		n := proto.Clone(m)
		n.Reset()
		if e = proto.Unmarshal(b, n); e != nil || !proto.Equal(m, n) {
			t.Fatal("wire mismatch", e, m, n)
		}
	}
}
