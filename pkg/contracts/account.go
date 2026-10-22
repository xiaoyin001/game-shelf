// 文件说明：定义独立账号注册/验证契约；保留请求序号和原有网关凭据协议。
package contracts

import "github.com/golang/protobuf/proto"

const (
	LGDB_RegisterAccountReq  uint32 = 500005
	DBLG_RegisterAccountResp uint32 = 500006
	LGDB_VerifyAccountReq    uint32 = 500007
	DBLG_VerifyAccountResp   uint32 = 500008
	AccountOK                int32  = iota + 100
	AccountInvalid
	AccountExists
	AccountBusy
	AccountUnavailable
)

// AccountRequest 只供 login 到 db 的可信本机内部连接使用。
type AccountRequest struct {
	Account  string `protobuf:"bytes,1,opt,name=account,proto3" json:"account,omitempty"`
	Password string `protobuf:"bytes,2,opt,name=password,proto3" json:"-"`
}

func (m *AccountRequest) Reset()         { *m = AccountRequest{} }
func (m *AccountRequest) String() string { return "AccountRequest{credentials:redacted}" }
func (*AccountRequest) ProtoMessage()    {}

// AccountReply 不包含密码或哈希，失败也不返回身份。
type AccountReply struct {
	Code    int32  `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
	UserID  string `protobuf:"bytes,2,opt,name=user_id,json=userId,proto3" json:"user_id,omitempty"`
	Account string `protobuf:"bytes,3,opt,name=account,proto3" json:"account,omitempty"`
}

func (m *AccountReply) Reset()         { *m = AccountReply{} }
func (m *AccountReply) String() string { return proto.CompactTextString(m) }
func (*AccountReply) ProtoMessage()    {}
