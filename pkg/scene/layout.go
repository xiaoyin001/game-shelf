// 文件说明：内嵌唯一的静态实体布局，服务端碰撞与浏览器绘制读取同一数据。
package scene

import (
	"embed"
	"encoding/json"
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

// Node 定义可采集资源的固定位置，与同 ID 的实体保持一致。
type Node struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

// NPC 与同 ID 实体共用坐标，Range 为包含高度的交互距离。
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

// Point 是无刚体道具的地面格。
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
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
