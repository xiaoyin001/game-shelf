// 文件说明：验证装饰白名单、收藏解锁、唯一槽位及嵌套存档的副本隔离。
package fieldbook

import (
	"game-shelf/pkg/contracts"
	"testing"
)

func TestCampValidationAndDeepCopy(t *testing.T) {
	found := []string{"bell", "letter", "flute"}
	bad := [][]*contracts.CampSlot{
		{nil},
		{{SlotID: "unknown", DecorationID: "bell-lantern"}},
		{{SlotID: "left", DecorationID: "unknown"}},
		{{SlotID: "left", DecorationID: "bell-lantern"}, {SlotID: "left", DecorationID: "lake-vase"}},
		{{SlotID: "left", DecorationID: "bell-lantern"}, {SlotID: "center", DecorationID: "bell-lantern"}},
	}
	for _, camp := range bad {
		if Validate(&contracts.Journal{Found: found, Camp: camp}) == nil {
			t.Fatal("accepted corrupt camp", camp)
		}
	}
	locked := &contracts.Journal{Camp: []*contracts.CampSlot{{SlotID: "left", DecorationID: "bell-lantern"}}}
	if Validate(locked) == nil {
		t.Fatal("unearned camp accepted")
	}
	j := &contracts.Journal{Found: found, Camp: []*contracts.CampSlot{{SlotID: "left", DecorationID: "bell-lantern"}, {SlotID: "center", DecorationID: "lake-vase"}, {SlotID: "right", DecorationID: "wind-chime"}}}
	if err := Validate(j); err != nil {
		t.Fatal(err)
	}
	view := View(j)
	view.Camp[0].DecorationID = "changed"
	copy := Clone(j)
	copy.Camp[1].SlotID = "changed"
	if j.Camp[0].DecorationID != "bell-lantern" || j.Camp[1].SlotID != "center" {
		t.Fatal("shared nested camp")
	}
	if len(view.Decorations) != 3 || !view.Decorations[2].Unlocked {
		t.Fatal("unlock not derived from collection")
	}
}
