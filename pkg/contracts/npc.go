// 文件说明：定义仅发给交互者的 NPC 对话与本轮委托进度，不混入公共世界快照。
package contracts

import "github.com/golang/protobuf/proto"

// WorldQuest 的进度和奖励由世界裁定，空状态表示尚未领取。
type WorldQuest struct {
	Status   string `protobuf:"bytes,1,opt,name=status,proto3" json:"status"`
	Progress int32  `protobuf:"varint,2,opt,name=progress,proto3" json:"progress"`
	Goal     int32  `protobuf:"varint,3,opt,name=goal,proto3" json:"goal"`
	Reward   int32  `protobuf:"varint,4,opt,name=reward,proto3" json:"reward"`
}

func (m *WorldQuest) Reset()         { *m = WorldQuest{} }
func (m *WorldQuest) String() string { return proto.CompactTextString(m) }
func (*WorldQuest) ProtoMessage()    {}

// WorldDialog 只在交互响应中出现，避免轮询重新打开已关闭的面板。
type WorldDialog struct {
	NPCID   string          `protobuf:"bytes,1,opt,name=npc_id,json=npcId,proto3" json:"npc_id"`
	Name    string          `protobuf:"bytes,2,opt,name=name,proto3" json:"name"`
	Text    string          `protobuf:"bytes,3,opt,name=text,proto3" json:"text"`
	Choices []*DialogChoice `protobuf:"bytes,4,rep,name=choices,proto3" json:"choices"`
}

func (m *WorldDialog) Reset()         { *m = WorldDialog{} }
func (m *WorldDialog) String() string { return proto.CompactTextString(m) }
func (*WorldDialog) ProtoMessage()    {}

// DialogChoice 使用固定标识；客户端不能提交任意对话或奖励。
type DialogChoice struct {
	ID    string `protobuf:"bytes,1,opt,name=id,proto3" json:"id"`
	Label string `protobuf:"bytes,2,opt,name=label,proto3" json:"label"`
}

func (m *DialogChoice) Reset()         { *m = DialogChoice{} }
func (m *DialogChoice) String() string { return proto.CompactTextString(m) }
func (*DialogChoice) ProtoMessage()    {}
