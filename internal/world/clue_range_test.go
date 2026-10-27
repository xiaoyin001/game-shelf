// 文件说明：保留真实浏览器松键余量的精确位置，验证可见提示不放宽调查距离。
package world

import (
	"game-shelf/pkg/contracts"
	"testing"
	"time"
)

func TestClueRangeAfterResidualMovement(t *testing.T) {
	now := time.Unix(1000, 0)
	store := newMemoryBooks()
	store.data["a"] = &contracts.Journal{Revision: 1, Active: "bell"}
	w := NewPersistent(store)
	enter(w, "a", "a", now)
	for _, point := range [][2]float64{{632.868032, 533.409808}, {634.001, 520}} {
		bookPosition(w, "a", point[0], point[1], 0)
		if reply := command(w, "a", "a", "explore", now); reply.Error != "clue_out_of_range" || w.players["a"].journal.Step != 0 {
			t.Fatal("out-of-range clue advanced", point, reply)
		}
	}
	bookPosition(w, "a", 634, 520, 0)
	if reply := command(w, "a", "a", "explore", now); reply.Error != "" || reply.Book.Trail.Step != 2 {
		t.Fatal("exact 34 boundary rejected", reply)
	}
}
