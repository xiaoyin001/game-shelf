// 文件说明：以服务端时间推进圆柱角色的重力、支撑和分步水平碰撞，不接收客户端高度。
package world

import (
	"game-shelf/pkg/scene"
	"math"
	"sort"
	"time"
)

const (
	playerRadius = 20.0
	playerHeight = 66.0
	jumpSpeed    = 380.0
	gravity      = 800.0
	ceiling      = 512.0
	epsilon      = 1e-6
)

// overlaps 判定角色足迹与圆柱/盒体的交叠，接触边缘不算穿入。
func overlaps(x, y float64, e scene.Entity) bool {
	if e.Radius > 0 {
		return math.Hypot(x-e.X, y-e.Y) < playerRadius+e.Radius-epsilon
	}
	nx := math.Max(e.X-e.HalfX, math.Min(e.X+e.HalfX, x))
	ny := math.Max(e.Y-e.HalfY, math.Min(e.Y+e.HalfY, y))
	return math.Hypot(x-nx, y-ny) < playerRadius-epsilon
}

// intervals 枚举当前足迹下的实体垂直区间，玩家身体也属于实体。
func (w *World) intervals(p *player, x, y float64, visit func(float64, float64)) {
	for _, e := range w.layout.Entities {
		if overlaps(x, y, e) {
			visit(0, e.Height)
		}
	}
	for id, other := range w.players {
		if id != p.state.ID && math.Hypot(x-other.state.X, y-other.state.Y) < 2*playerRadius-epsilon {
			visit(other.state.Z, other.state.Z+playerHeight)
		}
	}
}
func (w *World) blocked(p *player, x, y float64) bool {
	if x < playerRadius-epsilon || x > width-playerRadius+epsilon || y < playerRadius-epsilon || y > height-playerRadius+epsilon {
		return true
	}
	blocked := false
	w.intervals(p, x, y, func(bottom, top float64) {
		if p.state.Z < top-epsilon && p.state.Z+playerHeight > bottom+epsilon {
			blocked = true
		}
	})
	return blocked
}
func (w *World) supported(p *player) bool {
	if p.state.Z <= epsilon {
		return true
	}
	supported := false
	w.intervals(p, p.state.X, p.state.Y, func(_, top float64) {
		if math.Abs(p.state.Z-top) <= epsilon {
			supported = true
		}
	})
	return supported
}
func (w *World) advance(now time.Time) {
	if w.lastPhysics.IsZero() {
		w.lastPhysics = now
		return
	}
	remaining := now.Sub(w.lastPhysics).Seconds()
	if remaining <= 0 {
		return
	}
	w.lastPhysics = now
	ids := make([]string, 0, len(w.players))
	for id := range w.players {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return
	}
	sort.Strings(ids)
	// 租约五秒内的真实时间分步消费；消息数量不能加速重力或积攒水平移动。
	for remaining > epsilon {
		dt := math.Min(remaining, 1.0/60)
		remaining -= dt
		for _, id := range ids {
			w.fall(w.players[id], dt)
		}
	}
}
func (w *World) fall(p *player, dt float64) {
	if p.state.Grounded && w.supported(p) {
		p.vz = 0
		return
	}
	old := p.state.Z
	next := old + p.vz*dt - gravity*dt*dt/2
	p.vz -= gravity * dt
	p.state.Grounded = false
	if next > old {
		limit := ceiling - playerHeight
		w.intervals(p, p.state.X, p.state.Y, func(bottom, _ float64) {
			if old+playerHeight <= bottom+epsilon && next+playerHeight >= bottom {
				limit = math.Min(limit, bottom-playerHeight)
			}
		})
		if next >= limit {
			next = limit
			p.vz = 0
		}
	} else {
		floor := 0.0
		w.intervals(p, p.state.X, p.state.Y, func(_, top float64) {
			if old >= top-epsilon && next <= top {
				floor = math.Max(floor, top)
			}
		})
		if next <= floor {
			next = floor
			p.vz = 0
			p.state.Grounded = true
		}
	}
	p.state.Z = math.Max(0, next)
}
func (w *World) move(p *player, dx, dy float64, now time.Time) string {
	if math.IsNaN(dx) || math.IsNaN(dy) || math.IsInf(dx, 0) || math.IsInf(dy, 0) {
		return "invalid_direction"
	}
	length := math.Hypot(dx, dy)
	if math.IsInf(length, 0) {
		return "invalid_direction"
	}
	if length > 1 {
		dx /= length
		dy /= length
	}
	dt := math.Max(0, math.Min(now.Sub(p.lastMove).Seconds(), .15))
	if now.After(p.lastMove) {
		p.lastMove = now
	}
	mx, my := dx*speed*dt, dy*speed*dt
	steps := int(math.Ceil(math.Hypot(mx, my) / 3))
	for i := 0; i < steps; i++ {
		x := math.Max(playerRadius, math.Min(width-playerRadius, p.state.X+mx/float64(steps)))
		y := math.Max(playerRadius, math.Min(height-playerRadius, p.state.Y+my/float64(steps)))
		if !w.blocked(p, x, y) {
			p.state.X = x
			p.state.Y = y
			continue
		}
		// 沿未阻挡轴滑动，不把一次碰撞改成穿透或横向推走其他玩家。
		if !w.blocked(p, x, p.state.Y) {
			p.state.X = x
		}
		if !w.blocked(p, p.state.X, y) {
			p.state.Y = y
		}
	}
	if p.state.Grounded && !w.supported(p) {
		p.state.Grounded = false
	}
	return ""
}
