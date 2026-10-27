// 文件说明：在权威位置上推进私人寻宝，先同步保存再公布收藏结果。
package world

import (
	"context"
	"game-shelf/pkg/contracts"
	"game-shelf/pkg/fieldbook"
	"math"
)

// ProgressStore 是当前收藏读写所需的最小边界，不存临时世界状态。
type ProgressStore interface {
	Load(context.Context, string) (*contracts.Journal, error)
	Save(context.Context, string, *contracts.Journal) (*contracts.Journal, error)
}

func (w *World) acceptTrail(ctx context.Context, p *player) string {
	if p.journal.Active != "" || fieldbook.Next(p.journal) == "" {
		return ""
	}
	next := fieldbook.Clone(p.journal)
	next.Active = fieldbook.Next(next)
	return w.persistBook(ctx, p, next)
}
func (w *World) explore(ctx context.Context, p *player) string {
	target := fieldbook.Current(p.journal)
	if target == nil {
		return "no_active_trail"
	}
	if !p.state.Grounded || !w.supported(p) || math.Abs(p.state.Z-target.Z) > 10 || math.Hypot(math.Hypot(p.state.X-target.X, p.state.Y-target.Y), p.state.Z-target.Z) > 34 {
		return "clue_out_of_range"
	}
	next := fieldbook.Clone(p.journal)
	if next.Step == 0 {
		next.Step = 1
	} else {
		next.Found = append(next.Found, next.Active)
		next.Active = ""
		next.Step = 0
	}
	return w.persistBook(ctx, p, next)
}

// persistBook 在调用方已持有世界锁时使用；分片账号锁跨I/O持有，旧会话不能覆盖新会话。
func (w *World) persistBook(ctx context.Context, p *player, next *contracts.Journal) string {
	if w.books == nil {
		return "progress_unavailable"
	}
	w.mu.Unlock()
	saved, err := w.books.Save(ctx, p.state.ID, next)
	if err != nil {
		// 超时可能已经提交；只重读确认，不将本地草稿当成成功，也不覆盖版本。
		if loaded, loadErr := w.books.Load(ctx, p.state.ID); loadErr == nil && fieldbook.Validate(loaded) == nil {
			saved = loaded
		}
	}
	w.mu.Lock()
	if current := w.players[p.state.ID]; current != p || current.session != p.session {
		return "session_replaced_or_expired"
	}
	if saved != nil && fieldbook.Validate(saved) == nil {
		p.journal = fieldbook.Clone(saved)
	}
	if err != nil || fieldbook.Validate(saved) != nil {
		return "progress_unavailable"
	}
	return ""
}
