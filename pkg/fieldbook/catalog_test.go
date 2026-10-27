// 文件说明：验证有限收藏册的内容白名单、步骤约束与独立响应，不代替真实存储回归。
package fieldbook

import (
	"game-shelf/pkg/contracts"
	"testing"
)

func TestJournalValidationAndPrivateCopy(t *testing.T) {
	for _, bad := range []*contracts.Journal{nil, {Revision: -1}, {Found: []string{"flute"}}, {Found: []string{"bell", "bell"}}, {Active: "unknown"}, {Active: "bell", Step: 2}, {Active: "letter"}, {Step: 1}, {Found: []string{"bell", "letter", "flute"}, Active: "flute"}} {
		if Validate(bad) == nil {
			t.Fatalf("accepted corrupt journal: %+v", bad)
		}
	}
	j := &contracts.Journal{Found: []string{"bell"}, Active: "letter", Step: 1, Revision: 4}
	if err := Validate(j); err != nil {
		t.Fatal(err)
	}
	view := View(j)
	if !view.Collections[0].Found || view.Collections[0].Story == "" || view.Collections[1].Story != "" || view.Trail.Step != 2 {
		t.Fatal(view)
	}
	copy := Clone(j)
	copy.Found[0] = "changed"
	view.Trail.X = -999
	if j.Found[0] != "bell" || Current(j).X == -999 {
		t.Fatal("shared mutable data")
	}
	for _, entry := range Entries() {
		a, b := entry.Stops[0], entry.Stops[1]
		dx, dy := a.X-b.X, a.Y-b.Y
		if dx*dx+dy*dy <= 68*68 {
			t.Fatal("two investigations can overlap", entry.ID)
		}
	}
}
