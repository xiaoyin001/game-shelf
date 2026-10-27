// 文件说明：定义账号收藏存档与仅发给本人的线索册，客户端不能提交存档内容。
package contracts

import "github.com/golang/protobuf/proto"

const (
	LBDB_LoadBookReq  uint32 = 600003
	DBLB_LoadBookResp uint32 = 600004
	LBDB_SaveBookReq  uint32 = 600005
	DBLB_SaveBookResp uint32 = 600006
)

// Journal 只存有限探索状态；Revision是保存时的预期版本，成功后递增。
type Journal struct {
	Revision int64       `protobuf:"varint,1,opt,name=revision,proto3" json:"revision" bson:"revision"`
	Found    []string    `protobuf:"bytes,2,rep,name=found,proto3" json:"found" bson:"found"`
	Active   string      `protobuf:"bytes,3,opt,name=active,proto3" json:"active" bson:"active"`
	Step     int32       `protobuf:"varint,4,opt,name=step,proto3" json:"step" bson:"step"`
	Camp     []*CampSlot `protobuf:"bytes,5,rep,name=camp,proto3" json:"camp" bson:"camp,omitempty"`
}

func (m *Journal) Reset()         { *m = Journal{} }
func (m *Journal) String() string { return proto.CompactTextString(m) }
func (*Journal) ProtoMessage()    {}

// BookRequest 的身份仅来自lobby，存档只走可信内部TCP。
type BookRequest struct {
	UserID  string   `protobuf:"bytes,1,opt,name=user_id,json=userId,proto3" json:"user_id"`
	Journal *Journal `protobuf:"bytes,2,opt,name=journal,proto3" json:"journal,omitempty"`
}

func (m *BookRequest) Reset()         { *m = BookRequest{} }
func (m *BookRequest) String() string { return proto.CompactTextString(m) }
func (*BookRequest) ProtoMessage()    {}

type BookReply struct {
	Error   string   `protobuf:"bytes,1,opt,name=error,proto3" json:"error,omitempty"`
	Journal *Journal `protobuf:"bytes,2,opt,name=journal,proto3" json:"journal,omitempty"`
}

func (m *BookReply) Reset()         { *m = BookReply{} }
func (m *BookReply) String() string { return proto.CompactTextString(m) }
func (*BookReply) ProtoMessage()    {}

// FieldBook 的故事、名称和当前目标均由服务器从存档派生。
type FieldBook struct {
	Collections []*Collection `protobuf:"bytes,1,rep,name=collections,proto3" json:"collections"`
	Trail       *Trail        `protobuf:"bytes,2,opt,name=trail,proto3" json:"trail,omitempty"`
	Decorations []*Decoration `protobuf:"bytes,3,rep,name=decorations,proto3" json:"decorations"`
	Camp        []*CampSlot   `protobuf:"bytes,4,rep,name=camp,proto3" json:"camp"`
}

func (m *FieldBook) Reset()         { *m = FieldBook{} }
func (m *FieldBook) String() string { return proto.CompactTextString(m) }
func (*FieldBook) ProtoMessage()    {}

type Collection struct {
	ID    string `protobuf:"bytes,1,opt,name=id,proto3" json:"id"`
	Name  string `protobuf:"bytes,2,opt,name=name,proto3" json:"name"`
	Story string `protobuf:"bytes,3,opt,name=story,proto3" json:"story"`
	Found bool   `protobuf:"varint,4,opt,name=found,proto3" json:"found"`
}

func (m *Collection) Reset()         { *m = Collection{} }
func (m *Collection) String() string { return proto.CompactTextString(m) }
func (*Collection) ProtoMessage()    {}

type Trail struct {
	ID    string  `protobuf:"bytes,1,opt,name=id,proto3" json:"id"`
	Title string  `protobuf:"bytes,2,opt,name=title,proto3" json:"title"`
	Clue  string  `protobuf:"bytes,3,opt,name=clue,proto3" json:"clue"`
	X     float64 `protobuf:"fixed64,4,opt,name=x,proto3" json:"x"`
	Y     float64 `protobuf:"fixed64,5,opt,name=y,proto3" json:"y"`
	Z     float64 `protobuf:"fixed64,6,opt,name=z,proto3" json:"z"`
	Step  int32   `protobuf:"varint,7,opt,name=step,proto3" json:"step"`
	Total int32   `protobuf:"varint,8,opt,name=total,proto3" json:"total"`
}

func (m *Trail) Reset()         { *m = Trail{} }
func (m *Trail) String() string { return proto.CompactTextString(m) }
func (*Trail) ProtoMessage()    {}
