// 文件说明：测试专用地面绕障路径，只计算方向目标；真实位置仍须公开输入由服务器推进。
package navigation

import (
	"game-shelf/pkg/scene"
	"math"
	"sort"
)

func clear(l scene.Layout, p scene.Point, radius float64, bodies []scene.Point) bool {
	if p.X < radius || p.X > l.Width-radius || p.Y < radius || p.Y > l.Height-radius {
		return false
	}
	for _, e := range l.Entities {
		if e.Radius > 0 {
			if math.Hypot(p.X-e.X, p.Y-e.Y) < radius+e.Radius {
				return false
			}
		} else {
			x := math.Max(e.X-e.HalfX, math.Min(e.X+e.HalfX, p.X))
			y := math.Max(e.Y-e.HalfY, math.Min(e.Y+e.HalfY, p.Y))
			if math.Hypot(p.X-x, p.Y-y) < radius {
				return false
			}
		}
	}
	for _, b := range bodies {
		if math.Hypot(p.X-b.X, p.Y-b.Y) < radius+20-1e-6 {
			return false
		}
	}
	return true
}
func segment(l scene.Layout, a, b scene.Point, r float64, bodies []scene.Point) bool {
	n := int(math.Ceil(math.Hypot(a.X-b.X, a.Y-b.Y) / 3))
	for i := 0; i <= n; i++ {
		f := float64(i) / float64(max(1, n))
		if !clear(l, scene.Point{X: a.X + (b.X-a.X)*f, Y: a.Y + (b.Y-a.Y)*f}, r, bodies) {
			return false
		}
	}
	return true
}

// Route 使用共享真实碰撞布局。终点在目标30单位内，远小于拾取/返台范围；不跨过任何实体。
func Route(l scene.Layout, start, goal scene.Point, bodies []scene.Point) []scene.Point {
	const step = 20
	cols, rows := int(l.Width/step)-1, int(l.Height/step)-1
	point := func(i int) scene.Point {
		return scene.Point{X: float64((i%cols + 1) * step), Y: float64((i/cols + 1) * step)}
	}
	available := make([]bool, cols*rows)
	candidates := []int{}
	for i := range available {
		p := point(i)
		available[i] = clear(l, p, 36, bodies)
		if available[i] && math.Hypot(p.X-start.X, p.Y-start.Y) <= 80 {
			corner := scene.Point{X: p.X, Y: start.Y}
			if segment(l, start, corner, 20, bodies) && segment(l, corner, p, 20, bodies) {
				candidates = append(candidates, i)
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, b := point(candidates[i]), point(candidates[j])
		return math.Hypot(a.X-start.X, a.Y-start.Y) < math.Hypot(b.X-start.X, b.Y-start.Y)
	})
	if len(candidates) == 0 {
		return nil
	}
	root := candidates[0]
	parents := make([]int, len(available))
	for i := range parents {
		parents[i] = -2
	}
	parents[root] = -1
	queue := []int{root}
	end := -1
	for head := 0; head < len(queue); head++ {
		i := queue[head]
		p := point(i)
		if math.Hypot(p.X-goal.X, p.Y-goal.Y) <= 30 {
			end = i
			break
		}
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			x, y := i%cols+d[0], i/cols+d[1]
			if x < 0 || x >= cols || y < 0 || y >= rows {
				continue
			}
			j := y*cols + x
			if available[j] && parents[j] == -2 && segment(l, p, point(j), 36, bodies) {
				parents[j] = i
				queue = append(queue, j)
			}
		}
	}
	if end < 0 {
		return nil
	}
	path := []scene.Point{}
	for i := end; i >= 0; i = parents[i] {
		path = append(path, point(i))
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	rootPoint := path[0]
	out := []scene.Point{{X: rootPoint.X, Y: start.Y}, rootPoint}
	for i := 1; i < len(path); i++ {
		if len(out) >= 2 {
			a, b := out[len(out)-2], out[len(out)-1]
			if (a.X == b.X && b.X == path[i].X) || (a.Y == b.Y && b.Y == path[i].Y) {
				out[len(out)-1] = path[i]
				continue
			}
		}
		out = append(out, path[i])
	}
	return out
}
