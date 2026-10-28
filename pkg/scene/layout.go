// 文件说明：内嵌唯一的静态实体布局，服务端碰撞与浏览器绘制读取同一数据。
package scene

import (
	"embed"
	"encoding/json"
	"math"
)

// Files 只包含可公开的场景布局，不包含账号或运行配置。
//
//go:embed layout.json
var Files embed.FS

// Entity 的高度从地面起算；圆柱使用 Radius，盒体使用 HalfX/HalfY。
type Entity struct {
	ID     string  `json:"id"`
	Kind   string  `json:"kind"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Radius float64 `json:"radius,omitempty"`
	HalfX  float64 `json:"half_x,omitempty"`
	HalfY  float64 `json:"half_y,omitempty"`
	Height float64 `json:"height"`
	Scale  float64 `json:"scale"`
}

// Node 定义背景晶簇固定位置，与同 ID 的碰撞实体保持一致。
type Node struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

// NPC 保留公开布局解码兼容；当前列表为空，不再生成背景角色或身体。
type NPC struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Role  string  `json:"role"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Range float64 `json:"range"`
}

// DisplaySlot是无碰撞微光投影的位置，不属于实体家具。
type DisplaySlot struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}

// Camp 定义共享展台中心、交互距离和无碰撞投影槽位。
type Camp struct {
	X     float64       `json:"x"`
	Y     float64       `json:"y"`
	Range float64       `json:"range"`
	Slots []DisplaySlot `json:"slots"`
}

// ScatterJitter 为安全锚点的每轴最大抖动（场景单位），碰撞留白包含该偏移。
const ScatterJitter = 12

// Point 是道具安全锚点，Zone 为0–7的分散区域，每盘各选一处。
type Point struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Zone int     `json:"zone"`
}

// Water 与可见水面共用椭圆；SpeedFactor 是地面涉水速度与普通速度的比值。
type Water struct {
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	RadiusX     float64 `json:"radius_x"`
	RadiusY     float64 `json:"radius_y"`
	SpeedFactor float64 `json:"speed_factor"`
}

// Contains 包含水面边界；空水域不减速，非有限坐标不会被当作水面。
func (w Water) Contains(x, y float64) bool {
	if w.RadiusX <= 0 || w.RadiusY <= 0 {
		return false
	}
	dx, dy := (x-w.X)/w.RadiusX, (y-w.Y)/w.RadiusY
	return !math.IsInf(dx, 0) && !math.IsInf(dy, 0) && dx*dx+dy*dy <= 1
}

// Workshop 同时约束服务端距离和客户端场地投影，不添加静态阻挡体。
type Workshop struct {
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Range       float64 `json:"range"`
	PickupRange float64 `json:"pickup_range"`
	SpawnX      float64 `json:"spawn_x"`
	SpawnY      float64 `json:"spawn_y"`
	Spots       []Point `json:"spots"`
}

// Layout 是客户端绘制与服务器碰撞共用的公开场景版本。
type Layout struct {
	Version  string   `json:"version"`
	Width    float64  `json:"width"`
	Height   float64  `json:"height"`
	Entities []Entity `json:"entities"`
	Nodes    []Node   `json:"nodes"`
	NPCs     []NPC    `json:"npcs"`
	Camp     Camp     `json:"camp"`
	Workshop Workshop `json:"workshop"`
	Water    Water    `json:"water"`
}

// Default 每次解码独立副本，调用者不能修改其他世界或公开布局。
func Default() Layout {
	data, err := Files.ReadFile("layout.json")
	if err != nil {
		panic(err)
	}
	var layout Layout
	if err := json.Unmarshal(data, &layout); err != nil {
		panic(err)
	}
	return layout
}
