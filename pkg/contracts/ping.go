// 文件说明：定义通用心跳的 protobuf 消息，不提前引入业务协议。
package contracts

import "github.com/golang/protobuf/proto"

// PingReq 由连接发起方发送，时间使用毫秒。
type PingReq struct {
	UnixMillis int64 `protobuf:"varint,1,opt,name=unix_millis,json=unixMillis,proto3" json:"unix_millis"`
}

func (m *PingReq) Reset()         { *m = PingReq{} }
func (m *PingReq) String() string { return proto.CompactTextString(m) }
func (*PingReq) ProtoMessage()    {}

// PingResp 返回接收方时间，不能作为游戏移动预算。
type PingResp struct {
	UnixMillis int64 `protobuf:"varint,1,opt,name=unix_millis,json=unixMillis,proto3" json:"unix_millis"`
}

func (m *PingResp) Reset()         { *m = PingResp{} }
func (m *PingResp) String() string { return proto.CompactTextString(m) }
func (*PingResp) ProtoMessage()    {}
