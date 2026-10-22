// 文件说明：定义客户端和内部服务共享的协议号与 protobuf 消息契约。
package contracts

import "github.com/golang/protobuf/proto"

const (
	CS_PingReq       uint32 = 100001
	CS_GateAuthReq   uint32 = 100003
	CS_LobbyListReq  uint32 = 100005
	SC_PingResp      uint32 = 200001
	SC_GateAuthResp  uint32 = 200003
	SC_LobbyListResp uint32 = 200005
	SC_KickNotify    uint32 = 200007
	SC_ErrorNotify   uint32 = 200009

	GTLB_PingReq       uint32 = 400001
	LBGT_PingResp      uint32 = 400002
	GTLB_LobbyListReq  uint32 = 400003
	LBGT_LobbyListResp uint32 = 400004

	LGDB_PingReq  uint32 = 500001
	DBLG_PingResp uint32 = 500002
	// 500003/500004 原无密码 upsert 身份入口已移除，不再复用。

	LBDB_PingReq  uint32 = 600001
	DBLB_PingResp uint32 = 600002

	GTDB_PingReq  uint32 = 700001
	DBGT_PingResp uint32 = 700002

	GTGT_PingReq      uint32 = 800001
	GTGT_PingResp     uint32 = 800002
	GTGT_KickUserReq  uint32 = 800003
	GTGT_KickUserResp uint32 = 800004

	SYS_ErrorNotify uint32 = 900001
)

type LoginHTTPRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
	Channel  string `json:"channel"`
}

type LoginHTTPReply struct {
	UserID         string `json:"user_id"`
	Token          string `json:"token"`
	GateInstanceID string `json:"gate_instance_id"`
	GateAddr       string `json:"gate_addr"`
	// GateWebSocketURL 由发现中的 ready gate 公布给浏览器。
	GateWebSocketURL string `json:"gate_websocket_url,omitempty"`
	ExpireAt         string `json:"expire_at"`
}

type GateAuthReq struct {
	Token string `protobuf:"bytes,1,opt,name=token,proto3" json:"token,omitempty"`
}

func (m *GateAuthReq) Reset()         { *m = GateAuthReq{} }
func (m *GateAuthReq) String() string { return proto.CompactTextString(m) }
func (*GateAuthReq) ProtoMessage()    {}

type GateAuthResp struct {
	Code      int32  `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
	Message   string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
	UserID    string `protobuf:"bytes,3,opt,name=user_id,json=userId,proto3" json:"user_id,omitempty"`
	SessionID string `protobuf:"bytes,4,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
}

func (m *GateAuthResp) Reset()         { *m = GateAuthResp{} }
func (m *GateAuthResp) String() string { return proto.CompactTextString(m) }
func (*GateAuthResp) ProtoMessage()    {}

type LobbyListReq struct {
	UserID string `protobuf:"bytes,1,opt,name=user_id,json=userId,proto3" json:"user_id,omitempty"`
}

func (m *LobbyListReq) Reset()         { *m = LobbyListReq{} }
func (m *LobbyListReq) String() string { return proto.CompactTextString(m) }
func (*LobbyListReq) ProtoMessage()    {}

type LobbyListResp struct {
	UserID      string   `protobuf:"bytes,1,opt,name=user_id,json=userId,proto3" json:"user_id,omitempty"`
	OnlineCount int64    `protobuf:"varint,2,opt,name=online_count,json=onlineCount,proto3" json:"online_count,omitempty"`
	Rooms       []string `protobuf:"bytes,3,rep,name=rooms,proto3" json:"rooms,omitempty"`
}

func (m *LobbyListResp) Reset()         { *m = LobbyListResp{} }
func (m *LobbyListResp) String() string { return proto.CompactTextString(m) }
func (*LobbyListResp) ProtoMessage()    {}

type KickUserReq struct {
	UserID    string `protobuf:"bytes,1,opt,name=user_id,json=userId,proto3" json:"user_id,omitempty"`
	SessionID string `protobuf:"bytes,2,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
	Reason    string `protobuf:"bytes,3,opt,name=reason,proto3" json:"reason,omitempty"`
}

func (m *KickUserReq) Reset()         { *m = KickUserReq{} }
func (m *KickUserReq) String() string { return proto.CompactTextString(m) }
func (*KickUserReq) ProtoMessage()    {}

type KickUserResp struct {
	Code    int32  `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
	Message string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
}

func (m *KickUserResp) Reset()         { *m = KickUserResp{} }
func (m *KickUserResp) String() string { return proto.CompactTextString(m) }
func (*KickUserResp) ProtoMessage()    {}

type KickNotify struct {
	Reason string `protobuf:"bytes,1,opt,name=reason,proto3" json:"reason,omitempty"`
}

func (m *KickNotify) Reset()         { *m = KickNotify{} }
func (m *KickNotify) String() string { return proto.CompactTextString(m) }
func (*KickNotify) ProtoMessage()    {}

type ErrorNotify struct {
	Code    int32  `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
	Message string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
}

func (m *ErrorNotify) Reset()         { *m = ErrorNotify{} }
func (m *ErrorNotify) String() string { return proto.CompactTextString(m) }
func (*ErrorNotify) ProtoMessage()    {}
