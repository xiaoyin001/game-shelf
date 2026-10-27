// 文件说明：公开本房等待、固定成员与重连状态，不向房外广播房间码。
package contracts

import "github.com/golang/protobuf/proto"

// ChallengeRoom 与局快照分开：等待时尚无Round，重开后需再次准备。
type ChallengeRoom struct {
	ID             string        `protobuf:"bytes,1,opt,name=id,proto3" json:"id"`
	Code           string        `protobuf:"bytes,2,opt,name=code,proto3" json:"code"`
	Mode           string        `protobuf:"bytes,3,opt,name=mode,proto3" json:"mode"`
	HostID         string        `protobuf:"bytes,4,opt,name=host_id,proto3" json:"host_id"`
	Phase          string        `protobuf:"bytes,5,opt,name=phase,proto3" json:"phase"`
	Eligibility    string        `protobuf:"bytes,6,opt,name=eligibility,proto3" json:"eligibility"`
	PracticeReason string        `protobuf:"bytes,7,opt,name=practice_reason,proto3" json:"practice_reason"`
	Members        []*RoomMember `protobuf:"bytes,8,rep,name=members,proto3" json:"members"`
}

func (m *ChallengeRoom) Reset()         { *m = ChallengeRoom{} }
func (m *ChallengeRoom) String() string { return proto.CompactTextString(m) }
func (*ChallengeRoom) ProtoMessage()    {}

// RoomMember 的重连剩余时间由服务器单调时钟产生，Quit表示本局席位已放弃。
type RoomMember struct {
	ID          string `protobuf:"bytes,1,opt,name=id,proto3" json:"id"`
	Name        string `protobuf:"bytes,2,opt,name=name,proto3" json:"name"`
	Ready       bool   `protobuf:"varint,3,opt,name=ready,proto3" json:"ready"`
	Online      bool   `protobuf:"varint,4,opt,name=online,proto3" json:"online"`
	Quit        bool   `protobuf:"varint,5,opt,name=quit,proto3" json:"quit"`
	ReconnectMS int64  `protobuf:"varint,6,opt,name=reconnect_ms,proto3" json:"reconnect_ms"`
}

func (m *RoomMember) Reset()         { *m = RoomMember{} }
func (m *RoomMember) String() string { return proto.CompactTextString(m) }
func (*RoomMember) ProtoMessage()    {}
