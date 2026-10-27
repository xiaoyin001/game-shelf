// 文件说明：定义收藏解锁的三种装饰与展台存档约束，不引入可消费或可交易资产。
package fieldbook

import (
	"errors"
	"game-shelf/pkg/contracts"
	"game-shelf/pkg/scene"
)

// MaxRevision 是可读取的最高版本；存储在达到该值时拒绝继续写入。
const MaxRevision int64 = 1000000

// Decorations 返回独立的解锁视图，不把摆件当作可消费库存。
func Decorations(j *contracts.Journal) []*contracts.Decoration {
	names := [3]string{"归途铃灯", "湖色花盏", "晶叶风铃"}
	ids := [3]string{"bell-lantern", "lake-vase", "wind-chime"}
	var result []*contracts.Decoration
	for i, id := range ids {
		result = append(result, &contracts.Decoration{ID: id, Name: names[i], Unlocked: i < len(j.Found)})
	}
	return result
}

// Unlocked 只认可当前收藏已经解锁的固定装饰。
func Unlocked(j *contracts.Journal, id string) bool {
	for _, d := range Decorations(j) {
		if d.ID == id {
			return d.Unlocked
		}
	}
	return false
}

// ValidSlot 与服务端布局、浏览器绘制读取同一份槽位定义。
func ValidSlot(id string) bool {
	for _, s := range scene.Default().Camp.Slots {
		if s.ID == id {
			return true
		}
	}
	return false
}
func validateCamp(j *contracts.Journal) error {
	if len(j.Camp) > len(scene.Default().Camp.Slots) {
		return errors.New("invalid_camp")
	}
	slots, items := map[string]bool{}, map[string]bool{}
	for _, s := range j.Camp {
		if s == nil || !ValidSlot(s.SlotID) || slots[s.SlotID] || items[s.DecorationID] || !Unlocked(j, s.DecorationID) {
			return errors.New("invalid_camp")
		}
		slots[s.SlotID], items[s.DecorationID] = true, true
	}
	return nil
}
