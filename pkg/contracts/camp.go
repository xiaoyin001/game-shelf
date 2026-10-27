// 文件说明：定义固定槽位微光展台与公共布置，不包含私人寻宝线索或收藏存档版本。
package contracts

import "github.com/golang/protobuf/proto"

// CampSlot 仅记录白名单槽位和装饰标识，不包含拥有者或任意坐标。
type CampSlot struct {
	SlotID       string `protobuf:"bytes,1,opt,name=slot_id,json=slotId,proto3" json:"slot_id" bson:"slot_id"`
	DecorationID string `protobuf:"bytes,2,opt,name=decoration_id,json=decorationId,proto3" json:"decoration_id" bson:"decoration_id"`
}

func (m *CampSlot) Reset()         { *m = CampSlot{} }
func (m *CampSlot) String() string { return proto.CompactTextString(m) }
func (*CampSlot) ProtoMessage()    {}

// Decoration 的解锁状态从服务器保存的收藏派生，不接受客户端修改。
type Decoration struct {
	ID       string `protobuf:"bytes,1,opt,name=id,proto3" json:"id"`
	Name     string `protobuf:"bytes,2,opt,name=name,proto3" json:"name"`
	Unlocked bool   `protobuf:"varint,3,opt,name=unlocked,proto3" json:"unlocked"`
}

func (m *Decoration) Reset()         { *m = Decoration{} }
func (m *Decoration) String() string { return proto.CompactTextString(m) }
func (*Decoration) ProtoMessage()    {}

// CampDisplay 仅包含玩家自愿展示的布置，观看不获得编辑权限。
type CampDisplay struct {
	OwnerID string      `protobuf:"bytes,1,opt,name=owner_id,json=ownerId,proto3" json:"owner_id"`
	Name    string      `protobuf:"bytes,2,opt,name=name,proto3" json:"name"`
	Slots   []*CampSlot `protobuf:"bytes,3,rep,name=slots,proto3" json:"slots"`
}

func (m *CampDisplay) Reset()         { *m = CampDisplay{} }
func (m *CampDisplay) String() string { return proto.CompactTextString(m) }
func (*CampDisplay) ProtoMessage()    {}
