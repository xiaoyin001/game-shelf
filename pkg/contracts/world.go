// 文件说明：定义世界在线与移动输入的内部 protobuf 契约。
package contracts

import "github.com/golang/protobuf/proto"

// 世界消息使用 gate 与 lobby 原有协议号段。
const (
	GTLB_WorldReq  uint32 = 400005
	LBGT_WorldResp uint32 = 400006
)

// WorldReq 的身份由 gate 填充，不能从浏览器请求透传。
type WorldReq struct {
	UserID    string  `protobuf:"bytes,1,opt,name=user_id,json=userId,proto3" json:"user_id"`
	SessionID string  `protobuf:"bytes,2,opt,name=session_id,json=sessionId,proto3" json:"session_id"`
	Action    string  `protobuf:"bytes,3,opt,name=action,proto3" json:"action"`
	Name      string  `protobuf:"bytes,4,opt,name=name,proto3" json:"name"`
	DX        float64 `protobuf:"fixed64,5,opt,name=dx,proto3" json:"dx"`
	DY        float64 `protobuf:"fixed64,6,opt,name=dy,proto3" json:"dy"`
	Text      string  `protobuf:"bytes,7,opt,name=text,proto3" json:"text"`
	// 旧 NPC/展台字段号 8–11 停用，不复用。
	ItemID   string `protobuf:"bytes,12,opt,name=item_id,proto3" json:"item_id"`
	RoundID  string `protobuf:"bytes,13,opt,name=round_id,proto3" json:"round_id"`
	RoomCode string `protobuf:"bytes,14,opt,name=room_code,proto3" json:"room_code"`
	Ready    bool   `protobuf:"varint,15,opt,name=ready,proto3" json:"ready"`
	RoomID   string `protobuf:"bytes,16,opt,name=room_id,proto3" json:"room_id"`
	Mode     string `protobuf:"bytes,17,opt,name=mode,proto3" json:"mode"`
	Day      string `protobuf:"bytes,18,opt,name=day,proto3" json:"day"`
}

func (m *WorldReq) Reset()         { *m = WorldReq{} }
func (m *WorldReq) String() string { return proto.CompactTextString(m) }
func (*WorldReq) ProtoMessage()    {}

// WorldResp 返回当前权威快照或业务错误。
type WorldResp struct {
	Error string      `protobuf:"bytes,1,opt,name=error,proto3" json:"error,omitempty"`
	State *WorldState `protobuf:"bytes,2,opt,name=state,proto3" json:"state,omitempty"`
	// 旧 Quest/Dialog/Book 字段号 3–5 停用，不复用。
	Challenge  *Challenge     `protobuf:"bytes,6,opt,name=challenge,proto3" json:"challenge,omitempty"`
	Room       *ChallengeRoom `protobuf:"bytes,7,opt,name=room,proto3" json:"room,omitempty"`
	Settlement *Settlement    `protobuf:"bytes,8,opt,name=settlement,proto3" json:"settlement,omitempty"`
	Board      *Leaderboard   `protobuf:"bytes,9,opt,name=board,proto3" json:"board,omitempty"`
}

func (m *WorldResp) Reset()         { *m = WorldResp{} }
func (m *WorldResp) String() string { return proto.CompactTextString(m) }
func (*WorldResp) ProtoMessage()    {}

// WorldState 是独立复制的在线状态，不暴露凭据。
type WorldState struct {
	SceneVersion string         `protobuf:"bytes,7,opt,name=scene_version,json=sceneVersion,proto3" json:"scene_version"`
	TimeMS       int64          `protobuf:"varint,1,opt,name=time_ms,json=timeMs,proto3" json:"time_ms"`
	Width        float64        `protobuf:"fixed64,2,opt,name=width,proto3" json:"width"`
	Height       float64        `protobuf:"fixed64,3,opt,name=height,proto3" json:"height"`
	Players      []*WorldPlayer `protobuf:"bytes,4,rep,name=players,proto3" json:"players"`
	Chats        []*WorldChat   `protobuf:"bytes,6,rep,name=chats,proto3" json:"chats"`
	Nodes        []*WorldNode   `protobuf:"bytes,5,rep,name=nodes,proto3" json:"nodes"`
	// 原 Camps 字段号 8 停用。
}

func (m *WorldState) Reset()         { *m = WorldState{} }
func (m *WorldState) String() string { return proto.CompactTextString(m) }
func (*WorldState) ProtoMessage()    {}

// WorldPlayer 仅包含对其他玩家可见的当前属性。
type WorldPlayer struct {
	ID   string  `protobuf:"bytes,1,opt,name=id,proto3" json:"id"`
	Name string  `protobuf:"bytes,2,opt,name=name,proto3" json:"name"`
	X    float64 `protobuf:"fixed64,3,opt,name=x,proto3" json:"x"`
	Y    float64 `protobuf:"fixed64,4,opt,name=y,proto3" json:"y"`
	// 原采集分数字段号 5 停用；配对分数只在局快照中。
	Z        float64 `protobuf:"fixed64,6,opt,name=z,proto3" json:"z"`
	Grounded bool    `protobuf:"varint,7,opt,name=grounded,proto3" json:"grounded"`
}

func (m *WorldPlayer) Reset()         { *m = WorldPlayer{} }
func (m *WorldPlayer) String() string { return proto.CompactTextString(m) }
func (*WorldPlayer) ProtoMessage()    {}

// WorldNode 仅为仍有碰撞的晶簇背景；旧冷却字段号 4 停用。
type WorldNode struct {
	ID string  `protobuf:"bytes,1,opt,name=id,proto3" json:"id"`
	X  float64 `protobuf:"fixed64,2,opt,name=x,proto3" json:"x"`
	Y  float64 `protobuf:"fixed64,3,opt,name=y,proto3" json:"y"`
}

func (m *WorldNode) Reset()         { *m = WorldNode{} }
func (m *WorldNode) String() string { return proto.CompactTextString(m) }
func (*WorldNode) ProtoMessage()    {}

// WorldChat 使用递增编号供客户端去重。
type WorldChat struct {
	ID   int64  `protobuf:"varint,1,opt,name=id,proto3" json:"id"`
	Name string `protobuf:"bytes,2,opt,name=name,proto3" json:"name"`
	Text string `protobuf:"bytes,3,opt,name=text,proto3" json:"text"`
}

func (m *WorldChat) Reset()         { *m = WorldChat{} }
func (m *WorldChat) String() string { return proto.CompactTextString(m) }
func (*WorldChat) ProtoMessage()    {}
