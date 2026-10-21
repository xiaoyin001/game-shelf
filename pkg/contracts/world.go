// 文件说明：定义当前世界进入、轮询与离开的内部 protobuf 契约。
package contracts

import "github.com/golang/protobuf/proto"

// 世界消息使用 gate 与 lobby 原有协议号段。
const (
	GTLB_WorldReq  uint32 = 400005
	LBGT_WorldResp uint32 = 400006
)

// WorldReq 的身份由 gate 填充，不能从浏览器请求透传。
type WorldReq struct {
	UserID    string `protobuf:"bytes,1,opt,name=user_id,json=userId,proto3" json:"user_id"`
	SessionID string `protobuf:"bytes,2,opt,name=session_id,json=sessionId,proto3" json:"session_id"`
	Action    string `protobuf:"bytes,3,opt,name=action,proto3" json:"action"`
	Name      string `protobuf:"bytes,4,opt,name=name,proto3" json:"name"`
}

func (m *WorldReq) Reset()         { *m = WorldReq{} }
func (m *WorldReq) String() string { return proto.CompactTextString(m) }
func (*WorldReq) ProtoMessage()    {}

// WorldResp 返回当前权威快照或业务错误。
type WorldResp struct {
	Error string      `protobuf:"bytes,1,opt,name=error,proto3" json:"error,omitempty"`
	State *WorldState `protobuf:"bytes,2,opt,name=state,proto3" json:"state,omitempty"`
}

func (m *WorldResp) Reset()         { *m = WorldResp{} }
func (m *WorldResp) String() string { return proto.CompactTextString(m) }
func (*WorldResp) ProtoMessage()    {}

// WorldState 是独立复制的在线状态，不暴露凭据。
type WorldState struct {
	TimeMS  int64          `protobuf:"varint,1,opt,name=time_ms,json=timeMs,proto3" json:"time_ms"`
	Width   float64        `protobuf:"fixed64,2,opt,name=width,proto3" json:"width"`
	Height  float64        `protobuf:"fixed64,3,opt,name=height,proto3" json:"height"`
	Players []*WorldPlayer `protobuf:"bytes,4,rep,name=players,proto3" json:"players"`
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
}

func (m *WorldPlayer) Reset()         { *m = WorldPlayer{} }
func (m *WorldPlayer) String() string { return proto.CompactTextString(m) }
func (*WorldPlayer) ProtoMessage()    {}
