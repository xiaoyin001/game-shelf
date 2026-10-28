// 文件说明：验证唯一场景数据的实体尺寸、标识和资源对应，防止渲染/碰撞来源漂移。
package scene

import "testing"

func TestCanonicalLayout(t *testing.T) {
	layout := Default()
	if layout.Version == "" || layout.Width != 1200 || layout.Height != 800 {
		t.Fatal("invalid world")
	}
	ids := map[string]Entity{}
	kinds := map[string]bool{}
	for _, e := range layout.Entities {
		if _, ok := ids[e.ID]; ok {
			t.Fatal("duplicate entity")
		}
		if e.ID == "" || e.Height <= 0 || e.Scale <= 0 || (e.Radius <= 0 && (e.HalfX <= 0 || e.HalfY <= 0)) {
			t.Fatal("invalid body")
		}
		ids[e.ID] = e
		kinds[e.Kind] = true
	}
	for _, kind := range []string{"tree", "rock", "tent", "campfire", "crystal"} {
		if !kinds[kind] {
			t.Fatal("missing collider class", kind)
		}
	}
	for _, n := range layout.Nodes {
		e := ids[n.ID]
		if e.Kind != "crystal" || e.X != n.X || e.Y != n.Y {
			t.Fatal("node/body drift")
		}
	}
	if len(layout.NPCs) != 0 || kinds["npc"] {
		t.Fatal("retired NPC or collider remains")
	}
	if layout.Camp.X != 730 || layout.Camp.Y != 530 || layout.Camp.Range != 110 || len(layout.Camp.Slots) != 3 {
		t.Fatal("invalid camp display")
	}
	for i, slot := range layout.Camp.Slots {
		if slot.ID != []string{"left", "center", "right"}[i] || slot.Name == "" || slot.X != 700+float64(i)*30 || slot.Y != 530 {
			t.Fatal("camp slot drift", slot)
		}
	}
	copy := Default()
	copy.Entities[0].X = -999
	if Default().Entities[0].X == -999 {
		t.Fatal("shared mutable map")
	}
}
