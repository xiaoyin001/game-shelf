// 文件说明：在营地附近权威处理个人展台，移动/收起投影不消耗收藏或本轮分数。
package world

import (
	"context"
	"game-shelf/pkg/contracts"
	"game-shelf/pkg/fieldbook"
	"math"
)

func (w *World) decorate(ctx context.Context, p *player, slotID, decorationID string) string {
	if !fieldbook.ValidSlot(slotID) {
		return "invalid_camp_slot"
	}
	if decorationID != "" && !fieldbook.Unlocked(p.journal, decorationID) {
		return "decoration_locked"
	}
	camp := w.layout.Camp
	if !p.state.Grounded || !w.supported(p) || math.Hypot(math.Hypot(p.state.X-camp.X, p.state.Y-camp.Y), p.state.Z) > camp.Range {
		return "camp_out_of_range"
	}
	next := fieldbook.Clone(p.journal)
	bySlot := map[string]string{}
	for _, s := range next.Camp {
		if s.SlotID != slotID && s.DecorationID != decorationID {
			bySlot[s.SlotID] = s.DecorationID
		}
	}
	if decorationID != "" {
		bySlot[slotID] = decorationID
	}
	next.Camp = nil
	for _, s := range camp.Slots {
		if id := bySlot[s.ID]; id != "" {
			next.Camp = append(next.Camp, &contracts.CampSlot{SlotID: s.ID, DecorationID: id})
		}
	}
	return w.persistBook(ctx, p, next)
}
