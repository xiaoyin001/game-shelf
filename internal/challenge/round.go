// 文件说明：裁定固定三分钟的配对、遮挡、容量和连击；调用方负责串行化及提供权威位置。
package challenge

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	mathrand "math/rand/v2"
	"time"

	"game-shelf/pkg/contracts"
	"game-shelf/pkg/scene"
)

const (
	Duration     = 180 * time.Second
	Rules        = "pair-1"
	HandCapacity = 3
	TrayCapacity = 5
)

var kinds = [...]string{"mushroom", "pinecone", "crystal", "berry", "leaf", "shell"}

// Round 的所有修改由世界锁保护，捕获快照不共享可变指针。
type Round struct {
	state         contracts.Challenge
	area          scene.Workshop
	deadline      time.Time
	comboDeadline time.Time
}

// New 创建服务器生成身份与 UTC 日种子的单人局，不从客户端读取计分参数。
func New(id, name string, now time.Time, area scene.Workshop) *Round {
	return newRound("solo", []*contracts.ChallengeMember{{ID: id, Name: name}}, now, area)
}

// NewCoop 只接受两名不同的已认证成员，名册在开局时复制锁定。
func NewCoop(a, b contracts.WorldPlayer, now time.Time, area scene.Workshop) *Round {
	if a.ID == "" || b.ID == "" || a.ID == b.ID {
		panic("双人局必须有两名不同成员")
	}
	return newRound("duo", []*contracts.ChallengeMember{{ID: a.ID, Name: a.Name}, {ID: b.ID, Name: b.Name}}, now, area)
}
func newRound(mode string, members []*contracts.ChallengeMember, now time.Time, area scene.Workshop) *Round {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		panic(err)
	}
	r := &Round{area: area, deadline: now.Add(Duration), state: contracts.Challenge{
		ID: hex.EncodeToString(token[:]), Mode: mode, Day: now.UTC().Format("2006-01-02"), Rules: Rules,
		Phase: "running", StartMS: now.UnixMilli(), EndMS: now.Add(Duration).UnixMilli(), Multiplier: 10, Board: 1, RemainingMS: Duration.Milliseconds(), Members: members,
	}}
	r.populate()
	return r
}

// ReleaseHeld 只在成员放弃席位时使用，将尚未消除的手持归还原位，避免剩余玩家死盘。
func (r *Round) ReleaseHeld(id string) {
	for _, i := range r.state.Items {
		if i.Place == "hand" && i.OwnerID == id {
			i.Place = "ground"
			i.OwnerID = ""
			i.Slot = 0
		}
	}
	r.resetCombo()
}

func (r *Round) populate() {
	seed := sha256.Sum256([]byte(fmt.Sprintf("%s/%s/%s/%d", r.state.Rules, r.state.Day, r.state.Mode, r.state.Board)))
	rng := mathrand.New(mathrand.NewChaCha8(seed))
	spots := rng.Perm(len(r.area.Spots))
	r.state.Items = nil
	for group := 0; group < 4; group++ {
		top := rng.IntN(len(kinds))
		bottom := (top + 1 + rng.IntN(len(kinds)-1)) % len(kinds)
		for side := 0; side < 2; side++ {
			p := r.area.Spots[spots[group*2+side]]
			upper := fmt.Sprintf("%d-%d-%d-top", r.state.Board, group, side)
			lower := fmt.Sprintf("%d-%d-%d-base", r.state.Board, group, side)
			r.state.Items = append(r.state.Items,
				&contracts.ChallengeItem{ID: upper, Kind: kinds[top], X: p.X, Y: p.Y, Z: 24, Place: "ground"},
				&contracts.ChallengeItem{ID: lower, Kind: kinds[bottom], X: p.X, Y: p.Y, Z: 0, Place: "ground", BlockerID: upper},
			)
		}
	}
}

// Advance 先截止再受理动作；清盘不更改 EndMS。
func (r *Round) Advance(now time.Time) {
	if r.state.Phase != "running" {
		return
	}
	r.state.RemainingMS = max(0, r.deadline.Sub(now).Milliseconds())
	if !now.Before(r.deadline) {
		r.state.Phase = "finished"
		r.state.EndReason = "time_up"
		return
	}
	if !r.comboDeadline.IsZero() && now.After(r.comboDeadline) {
		r.resetCombo()
	}
	if !r.comboDeadline.IsZero() {
		r.state.ComboRemainingMS = max(0, r.comboDeadline.Sub(now).Milliseconds())
	}
}
func (r *Round) ID() string    { return r.state.ID }
func (r *Round) Phase() string { return r.state.Phase }
func (r *Round) Abandon() {
	if r.state.Phase == "running" {
		r.state.Phase = "finished"
		r.state.EndReason = "abandoned"
	}
}
func (r *Round) item(id string) *contracts.ChallengeItem {
	for _, i := range r.state.Items {
		if i.ID == id {
			return i
		}
	}
	return nil
}
func (r *Round) member(id string) *contracts.ChallengeMember {
	for _, m := range r.state.Members {
		if m.ID == id {
			return m
		}
	}
	return nil
}
func (r *Round) count(place, owner string) int {
	n := 0
	for _, i := range r.state.Items {
		if i.Place == place && (owner == "" || i.OwnerID == owner) {
			n++
		}
	}
	return n
}
func (r *Round) blocked(i *contracts.ChallengeItem) bool {
	b := r.item(i.BlockerID)
	return b != nil && b.Place == "ground"
}
func distance(p contracts.WorldPlayer, x, y, z float64) float64 {
	return math.Hypot(math.Hypot(p.X-x, p.Y-y), p.Z-z)
}
func (r *Round) atTable(p contracts.WorldPlayer) bool {
	return p.Grounded && distance(p, r.area.X, r.area.Y, 0) <= r.area.Range
}
func (r *Round) slot(place, owner string, capacity int) int32 {
	for s := 0; s < capacity; s++ {
		used := false
		for _, i := range r.state.Items {
			if i.Place == place && (owner == "" || i.OwnerID == owner) && i.Slot == int32(s) {
				used = true
				break
			}
		}
		if !used {
			return int32(s)
		}
	}
	return -1
}
func (r *Round) resetCombo() {
	r.state.Multiplier = 10
	r.state.ComboUntilMS = 0
	r.state.ComboRemainingMS = 0
	r.comboDeadline = time.Time{}
}

// Apply 的身份、坐标、站立状态只能由权威世界传入；局身份防止跨重开残留命令。
func (r *Round) Apply(action, roundID, itemID string, p contracts.WorldPlayer, now time.Time) string {
	r.Advance(now)
	if r.state.Phase != "running" {
		return "round_finished"
	}
	if roundID != r.state.ID {
		return "stale_round"
	}
	m := r.member(p.ID)
	if m == nil {
		return "not_a_member"
	}
	i := r.item(itemID)
	if i == nil {
		return "unknown_item"
	}
	switch action {
	case "pick":
		if i.Place != "ground" {
			return "item_unavailable"
		}
		if r.blocked(i) {
			return "item_blocked"
		}
		if !p.Grounded || distance(p, i.X, i.Y, i.Z) > r.area.PickupRange {
			return "item_out_of_range"
		}
		if r.count("hand", p.ID) >= HandCapacity {
			return "hand_full"
		}
		i.Slot = r.slot("hand", p.ID, HandCapacity)
		i.Place = "hand"
		i.OwnerID = p.ID
	case "put":
		if i.Place != "hand" || i.OwnerID != p.ID {
			return "item_not_held"
		}
		if !r.atTable(p) {
			return "table_out_of_range"
		}
		var mate *contracts.ChallengeItem
		for _, other := range r.state.Items {
			if other.Place == "tray" && other.Kind == i.Kind {
				mate = other
				break
			}
		}
		if mate == nil {
			if r.count("tray", "") >= TrayCapacity {
				return "tray_full"
			}
			i.Slot = r.slot("tray", "", TrayCapacity)
			i.Place = "tray"
			i.OwnerID = ""
		} else {
			if !r.comboDeadline.IsZero() && !now.After(r.comboDeadline) {
				r.state.Multiplier += 5
				if r.state.Multiplier > 30 {
					r.state.Multiplier = 30
				}
			} else {
				r.state.Multiplier = 10
			}
			r.state.Score += 100 * r.state.Multiplier / 10
			r.state.Pairs++
			m.Pairs++
			r.comboDeadline = now.Add(10 * time.Second)
			r.state.ComboUntilMS = r.comboDeadline.UnixMilli()
			r.state.ComboRemainingMS = (10 * time.Second).Milliseconds()
			remaining := make([]*contracts.ChallengeItem, 0, len(r.state.Items)-2)
			for _, other := range r.state.Items {
				if other.ID != i.ID && other.ID != mate.ID {
					remaining = append(remaining, other)
				}
			}
			r.state.Items = remaining
			if len(remaining) == 0 {
				r.state.Score += 500
				r.state.Boards++
				r.state.Board++
				r.populate()
			}
		}
	case "return":
		if i.Place == "tray" {
			if !r.atTable(p) {
				return "table_out_of_range"
			}
		} else if i.Place != "hand" || i.OwnerID != p.ID {
			return "item_not_held"
		}
		i.Place = "ground"
		i.OwnerID = ""
		i.Slot = 0
		r.resetCombo()
	default:
		return "unknown_action"
	}
	return ""
}

// Snapshot 深复制所有列表，测试或传输不能改写下一帧规则状态。
func (r *Round) Snapshot() *contracts.Challenge {
	s := r.state
	s.Items = make([]*contracts.ChallengeItem, 0, len(r.state.Items))
	s.Members = make([]*contracts.ChallengeMember, 0, len(r.state.Members))
	for _, i := range r.state.Items {
		v := *i
		v.Blocked = i.Place == "ground" && r.blocked(i)
		s.Items = append(s.Items, &v)
	}
	for _, m := range r.state.Members {
		v := *m
		s.Members = append(s.Members, &v)
	}
	return &s
}
