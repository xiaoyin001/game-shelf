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
	UserID       string  `protobuf:"bytes,1,opt,name=user_id,json=userId,proto3" json:"user_id"`
	SessionID    string  `protobuf:"bytes,2,opt,name=session_id,json=sessionId,proto3" json:"session_id"`
	Action       string  `protobuf:"bytes,3,opt,name=action,proto3" json:"action"`
	Name         string  `protobuf:"bytes,4,opt,name=name,proto3" json:"name"`
	DX           float64 `protobuf:"fixed64,5,opt,name=dx,proto3" json:"dx"`
	DY           float64 `protobuf:"fixed64,6,opt,name=dy,proto3" json:"dy"`
	Text         string  `protobuf:"bytes,7,opt,name=text,proto3" json:"text"`
	NPCID        string  `protobuf:"bytes,8,opt,name=npc_id,json=npcId,proto3" json:"npc_id"`
	ChoiceID     string  `protobuf:"bytes,9,opt,name=choice_id,json=choiceId,proto3" json:"choice_id"`
	SlotID       string  `protobuf:"bytes,10,opt,name=slot_id,json=slotId,proto3" json:"slot_id"`
	DecorationID string  `protobuf:"bytes,11,opt,name=decoration_id,json=decorationId,proto3" json:"decoration_id"`
}

func (m *WorldReq) Reset()         { *m = WorldReq{} }
func (m *WorldReq) String() string { return proto.CompactTextString(m) }
func (*WorldReq) ProtoMessage()    {}

// WorldResp 返回当前权威快照或业务错误。
type WorldResp struct {
	Error  string       `protobuf:"bytes,1,opt,name=error,proto3" json:"error,omitempty"`
	State  *WorldState  `protobuf:"bytes,2,opt,name=state,proto3" json:"state,omitempty"`
	Quest  *WorldQuest  `protobuf:"bytes,3,opt,name=quest,proto3" json:"quest,omitempty"`
	Dialog *WorldDialog `protobuf:"bytes,4,opt,name=dialog,proto3" json:"dialog,omitempty"`
	Book   *FieldBook   `protobuf:"bytes,5,opt,name=book,proto3" json:"book,omitempty"`
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
	Camps        []*CampDisplay `protobuf:"bytes,8,rep,name=camps,proto3" json:"camps"`
}

func (m *WorldState) Reset()         { *m = WorldState{} }
func (m *WorldState) String() string { return proto.CompactTextString(m) }
func (*WorldState) ProtoMessage()    {}

// WorldPlayer 仅包含对其他玩家可见的当前属性。
type WorldPlayer struct {
	ID       string  `protobuf:"bytes,1,opt,name=id,proto3" json:"id"`
	Name     string  `protobuf:"bytes,2,opt,name=name,proto3" json:"name"`
	X        float64 `protobuf:"fixed64,3,opt,name=x,proto3" json:"x"`
	Y        float64 `protobuf:"fixed64,4,opt,name=y,proto3" json:"y"`
	Score    int32   `protobuf:"varint,5,opt,name=score,proto3" json:"score"`
	Z        float64 `protobuf:"fixed64,6,opt,name=z,proto3" json:"z"`
	Grounded bool    `protobuf:"varint,7,opt,name=grounded,proto3" json:"grounded"`
}

func (m *WorldPlayer) Reset()         { *m = WorldPlayer{} }
func (m *WorldPlayer) String() string { return proto.CompactTextString(m) }
func (*WorldPlayer) ProtoMessage()    {}

// WorldNode 的 ReadyAtMS 表示可再次采集的服务器时间。
type WorldNode struct {
	ID        string  `protobuf:"bytes,1,opt,name=id,proto3" json:"id"`
	X         float64 `protobuf:"fixed64,2,opt,name=x,proto3" json:"x"`
	Y         float64 `protobuf:"fixed64,3,opt,name=y,proto3" json:"y"`
	ReadyAtMS int64   `protobuf:"varint,4,opt,name=ready_at_ms,json=readyAtMs,proto3" json:"ready_at_ms"`
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
