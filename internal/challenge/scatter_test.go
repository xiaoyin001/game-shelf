// 文件说明：验证每日分散坐标的公平性、八区覆盖、原位归还及规则隔离。
package challenge

import (
	"game-shelf/pkg/contracts"
	"game-shelf/pkg/scene"
	"math"
	"reflect"
	"testing"
	"time"
)

func TestScatterDeterministicDistinctAndReturnOrigin(t *testing.T) {
	area := scene.Default().Workshop
	now := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	previous := []*contracts.ChallengeItem(nil)
	for day := range 120 {
		n := now.AddDate(0, 0, day)
		r := New("a", "甲", n, area)
		other := New("b", "乙", n.Add(15*time.Hour), area)
		if !reflect.DeepEqual(r.Snapshot().Items, other.Snapshot().Items) {
			t.Fatal("same day differs")
		}
		if r.Snapshot().Rules != "pair-2" {
			t.Fatal("old rules mixed")
		}
		if reflect.DeepEqual(previous, r.Snapshot().Items) {
			t.Fatal("day did not change positions")
		}
		previous = r.Snapshot().Items
		for board := 0; board < 3; board++ {
			s := r.Snapshot()
			seen := map[int]bool{}
			minX, maxX, minY, maxY := 1200.0, 0.0, 800.0, 0.0
			for i, item := range s.Items {
				if i%2 == 1 {
					continue
				}
				zone := -1
				for _, p := range area.Spots {
					if math.Abs(p.X-item.X) <= scene.ScatterJitter && math.Abs(p.Y-item.Y) <= scene.ScatterJitter {
						zone = p.Zone
						break
					}
				}
				if zone < 0 || seen[zone] {
					t.Fatal("missing or duplicated scatter zone", item)
				}
				seen[zone] = true
				if math.Hypot(item.X-area.X, item.Y-area.Y) < 130 {
					t.Fatal("items still next to table")
				}
				minX, maxX = math.Min(minX, item.X), math.Max(maxX, item.X)
				minY, maxY = math.Min(minY, item.Y), math.Max(maxY, item.Y)
				base := s.Items[i+1]
				if base.X != item.X || base.Y != item.Y || base.BlockerID != item.ID {
					t.Fatal("stack drift")
				}
			}
			if len(seen) != 8 || maxX-minX <= 700 || maxY-minY <= 300 {
				t.Fatal("not whole-map spread")
			}
			item := s.Items[0]
			take(t, r, item, n)
			apply(t, r, "return", item, pose(item), n)
			restored := r.item(item.ID)
			if restored.Place != "ground" || restored.X != item.X || restored.Y != item.Y {
				t.Fatal("return moved origin")
			}
			old := r.Snapshot().Items
			for range 8 {
				solvePair(t, r, n)
			}
			if reflect.DeepEqual(old, r.Snapshot().Items) {
				t.Fatal("new board did not change")
			}
		}
	}
	solo := New("a", "甲", now, area)
	duo := NewCoop(contracts.WorldPlayer{ID: "a"}, contracts.WorldPlayer{ID: "b"}, now, area)
	if reflect.DeepEqual(solo.Snapshot().Items, duo.Snapshot().Items) {
		t.Fatal("mode seed not separated")
	}
}
