// 文件说明：覆盖精确截止、可解关卡、距离归属、容量和不重复计分，不用客户端时钟计算奖励。
package challenge

import (
	"game-shelf/pkg/contracts"
	"game-shelf/pkg/scene"
	"reflect"
	"testing"
	"time"
)

func setup() (*Round, time.Time) {
	n := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	return New("a", "甲", n, scene.Default().Workshop), n
}
func pose(i *contracts.ChallengeItem) contracts.WorldPlayer {
	return contracts.WorldPlayer{ID: "a", X: i.X, Y: i.Y, Grounded: true}
}
func apply(t *testing.T, r *Round, a string, i *contracts.ChallengeItem, p contracts.WorldPlayer, n time.Time) {
	t.Helper()
	if e := r.Apply(a, r.ID(), i.ID, p, n); e != "" {
		t.Fatalf("%s %s: %s", a, i.ID, e)
	}
}
func take(t *testing.T, r *Round, i *contracts.ChallengeItem, n time.Time) {
	apply(t, r, "pick", i, pose(i), n)
}
func put(t *testing.T, r *Round, i *contracts.ChallengeItem, n time.Time) {
	apply(t, r, "put", i, contracts.WorldPlayer{ID: "a", X: 700, Y: 350, Grounded: true}, n)
}
func solvePair(t *testing.T, r *Round, n time.Time) {
	t.Helper()
	var a, b *contracts.ChallengeItem
	s := r.Snapshot()
	for _, i := range s.Items {
		if i.Place != "ground" || i.Blocked {
			continue
		}
		for _, j := range s.Items {
			if i.ID != j.ID && j.Place == "ground" && !j.Blocked && j.Kind == i.Kind {
				a, b = i, j
				break
			}
		}
		if a != nil {
			break
		}
	}
	if a == nil {
		t.Fatal("generated board has no open pair")
	}
	take(t, r, a, n)
	take(t, r, b, n)
	put(t, r, a, n)
	put(t, r, b, n)
}
func TestDailyDeterministicSolvableAndImmutableDeadline(t *testing.T) {
	for day := 0; day < 120; day++ {
		r, n := setup()
		n = n.AddDate(0, 0, day)
		r = New("a", "甲", n, scene.Default().Workshop)
		other := New("b", "乙", n, scene.Default().Workshop)
		if !reflect.DeepEqual(r.Snapshot().Items, other.Snapshot().Items) {
			t.Fatal("account-dependent seed")
		}
		end := r.Snapshot().EndMS
		for board := 1; board <= 12; board++ {
			if r.Snapshot().Board != int32(board) {
				t.Fatal("board did not replenish")
			}
			for pair := 0; pair < 8; pair++ {
				solvePair(t, r, n)
			}
			if r.Snapshot().EndMS != end || r.Snapshot().Boards != int32(board) {
				t.Fatal("clear changed time or count")
			}
		}
	}
	r, n := setup()
	s := r.Snapshot()
	if s.EndMS-s.StartMS != 180000 {
		t.Fatal("not fixed 180 seconds")
	}
	r.Advance(n.Add(180*time.Second - time.Millisecond))
	if r.Phase() != "running" {
		t.Fatal("ended early")
	}
	r.Advance(n.Add(180 * time.Second))
	if r.Phase() != "finished" || r.Snapshot().EndReason != "time_up" {
		t.Fatal("deadline not exact")
	}
	if r.Apply("pick", r.ID(), s.Items[0].ID, pose(s.Items[0]), n.Add(180*time.Second)) != "round_finished" {
		t.Fatal("deadline accepted action")
	}
	if r.Snapshot().Score != 0 {
		t.Fatal("unpaired item penalized or scored")
	}
}
func TestWireTimestampCannotChangeAuthoritativeDeadline(t *testing.T) {
	r, n := setup()
	r.state.EndMS = n.UnixMilli() + 1
	r.Advance(n.Add(time.Second))
	if r.Phase() != "running" || r.Snapshot().RemainingMS != 179000 {
		t.Fatal("wire metadata controls timer")
	}
	r.state.EndMS = n.Add(time.Hour).UnixMilli()
	r.Advance(n.Add(180 * time.Second))
	if r.Phase() != "finished" || r.Snapshot().RemainingMS != 0 {
		t.Fatal("metadata extended deadline")
	}
}

func TestBlockingCapacityOwnershipAndReturn(t *testing.T) {
	r, n := setup()
	s := r.Snapshot()
	top, base := s.Items[0], s.Items[1]
	if e := r.Apply("pick", r.ID(), base.ID, pose(base), n); e != "item_blocked" {
		t.Fatal(e)
	}
	p := pose(top)
	p.X += 1000
	if e := r.Apply("pick", r.ID(), top.ID, p, n); e != "item_out_of_range" {
		t.Fatal(e)
	}
	if e := r.Apply("pick", "old", top.ID, pose(top), n); e != "stale_round" {
		t.Fatal(e)
	}
	p = pose(top)
	p.ID = "intruder"
	if e := r.Apply("pick", r.ID(), top.ID, p, n); e != "not_a_member" {
		t.Fatal(e)
	}
	take(t, r, top, n)
	take(t, r, base, n)
	take(t, r, s.Items[2], n)
	if e := r.Apply("pick", r.ID(), s.Items[4].ID, pose(s.Items[4]), n); e != "hand_full" {
		t.Fatal(e)
	}
	if e := r.Apply("pick", r.ID(), top.ID, pose(top), n); e != "item_unavailable" {
		t.Fatal(e)
	}
	if e := r.Apply("put", r.ID(), base.ID, contracts.WorldPlayer{ID: "a", X: 1, Y: 1, Grounded: true}, n); e != "table_out_of_range" {
		t.Fatal(e)
	}
	apply(t, r, "return", base, pose(base), n)
	apply(t, r, "return", top, pose(top), n)
	if !r.Snapshot().Items[1].Blocked {
		t.Fatal("return did not restore blocker")
	}
	if r.Snapshot().Score != 0 {
		t.Fatal("moving or returning scored")
	}
	snap := r.Snapshot()
	snap.Items[0].Kind = "forged"
	snap.Members[0].Pairs = 99
	if r.Snapshot().Items[0].Kind == "forged" || r.Snapshot().Members[0].Pairs != 0 {
		t.Fatal("snapshot aliases internal state")
	}
}
func TestComboClearBonusAndReset(t *testing.T) {
	r, n := setup()
	expected := []int32{100, 250, 450, 700, 1000, 1300, 1600, 2400}
	for i := range 8 {
		solvePair(t, r, n.Add(time.Duration(i)*time.Second))
		if r.Snapshot().Score != expected[i] {
			t.Fatal(i, r.Snapshot().Score)
		}
	}
	if r.Snapshot().Multiplier != 30 || r.Snapshot().Boards != 1 || len(r.Snapshot().Items) != 16 {
		t.Fatal("clear/multiplier")
	}
	solvePair(t, r, n.Add(17*time.Second))
	if r.Snapshot().Score != 2700 {
		t.Fatal("10 second inclusive combo failed")
	}
	solvePair(t, r, n.Add(28*time.Second))
	if r.Snapshot().Score != 2800 {
		t.Fatal("expired combo continued")
	}
	i := r.Snapshot().Items[0]
	for _, j := range r.Snapshot().Items {
		if j.Place == "ground" && !j.Blocked {
			i = j
			break
		}
	}
	take(t, r, i, n.Add(29*time.Second))
	put(t, r, i, n.Add(29*time.Second))
	apply(t, r, "return", i, contracts.WorldPlayer{ID: "a", X: 700, Y: 350, Grounded: true}, n.Add(29*time.Second))
	if r.Snapshot().Multiplier != 10 || r.Snapshot().ComboUntilMS != 0 {
		t.Fatal("return kept combo")
	}
	solvePair(t, r, n.Add(30*time.Second))
	if r.Snapshot().Score != 2900 {
		t.Fatal("return did not reset multiplier")
	}
}
func TestFullTrayMatchingStillAcceptedAndRejectedItemPreserved(t *testing.T) {
	r, n := setup()
	// 直接构造六类开放物品，验证容量边界而非依赖某一天的随机分布。
	r.state.Items = nil
	for k, kind := range kinds {
		for c := 0; c < 2; c++ {
			r.state.Items = append(r.state.Items, &contracts.ChallengeItem{ID: kind + string(rune('a'+c)), Kind: kind, Place: "ground", X: 700 + float64(k), Y: 350})
		}
	}
	for k := 0; k < 5; k++ {
		i := r.state.Items[k*2]
		take(t, r, i, n)
		put(t, r, i, n)
	}
	sixth := r.state.Items[10]
	take(t, r, sixth, n)
	if e := r.Apply("put", r.ID(), sixth.ID, pose(sixth), n); e != "tray_full" || sixth.Place != "hand" {
		t.Fatal("full tray lost held item", e)
	}
	match := r.state.Items[1]
	take(t, r, match, n)
	put(t, r, match, n)
	if r.Snapshot().Score != 100 || r.count("tray", "") != 4 {
		t.Fatal("full tray failed to match")
	}
	if e := r.Apply("put", r.ID(), match.ID, pose(match), n); e != "unknown_item" {
		t.Fatal("duplicate elimination accepted", e)
	}
	put(t, r, sixth, n)
	if r.Snapshot().Boards != 0 {
		t.Fatal("cleared with items outstanding")
	}
}
