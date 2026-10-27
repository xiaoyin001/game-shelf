// 文件说明：规范不可变成绩与无序组合键，拒绝跨模式、损坏计分和任意客户端成绩结构。
package scores

import (
	"encoding/hex"
	"errors"
	"game-shelf/internal/challenge"
	"game-shelf/pkg/contracts"
	"github.com/golang/protobuf/proto"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrInvalid = errors.New("invalid_score")
var ErrConflict = errors.New("score_conflict")

func validID(id string) bool {
	b, err := hex.DecodeString(id)
	return err == nil && len(b) == 16 && strings.ToLower(id) == id
}
func ValidDay(day string) bool {
	d, err := time.Parse("2006-01-02", day)
	return err == nil && d.Year() >= 2000 && d.Year() <= 2100 && d.Format("2006-01-02") == day
}
func ValidQuery(userID, mode, day string) bool {
	return validID(userID) && (mode == "solo" || mode == "duo") && ValidDay(day)
}

// Normalize 深复制并按固定账号排序；交换成员输入顺序不制造新组合。
func Normalize(r *contracts.ScoreRecord) (*contracts.ScoreRecord, string, error) {
	if r == nil || !validID(r.RoundID) || r.Rules != challenge.Rules || !ValidDay(r.Day) || r.EndMS-r.StartMS != challenge.Duration.Milliseconds() || time.UnixMilli(r.StartMS).UTC().Format("2006-01-02") != r.Day {
		return nil, "", ErrInvalid
	}
	count := 1
	if r.Mode == "duo" {
		count = 2
	} else if r.Mode != "solo" {
		return nil, "", ErrInvalid
	}
	steps := min(r.Pairs, int32(5))
	maxScore := steps*100 + steps*(steps-1)*25 + max(0, r.Pairs-5)*300 + r.Boards*500
	if len(r.Members) != count || r.Score < 0 || r.Score > 1000000 || r.Pairs < 0 || r.Pairs > 3000 || r.Boards != r.Pairs/8 || r.Score%50 != 0 || r.Score < r.Pairs*100+r.Boards*500 || r.Score > maxScore {
		return nil, "", ErrInvalid
	}
	pairs := int32(0)
	ids := make([]string, 0, count)
	for _, m := range r.Members {
		if m == nil || !validID(m.ID) || strings.TrimSpace(m.Name) != m.Name || m.Name == "" || !utf8.ValidString(m.Name) || utf8.RuneCountInString(m.Name) > 24 || m.Pairs < 0 || m.Pairs > r.Pairs {
			return nil, "", ErrInvalid
		}
		pairs += m.Pairs
		ids = append(ids, m.ID)
	}
	if pairs != r.Pairs || (count == 2 && ids[0] == ids[1]) {
		return nil, "", ErrInvalid
	}
	n := proto.Clone(r).(*contracts.ScoreRecord)
	sort.Slice(n.Members, func(i, j int) bool { return n.Members[i].ID < n.Members[j].ID })
	sort.Strings(ids)
	return n, strings.Join(ids, ":"), nil
}
