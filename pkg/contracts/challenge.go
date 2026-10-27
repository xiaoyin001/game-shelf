// 文件说明：定义限时配对的权威局面、物品位置和成员贡献，不接收客户端分数。
package contracts

import "github.com/golang/protobuf/proto"

// Challenge 是独立局的完整可见快照，物品已消除后不再发送。
type Challenge struct {
	ID               string             `protobuf:"bytes,1,opt,name=id,proto3" json:"id"`
	Mode             string             `protobuf:"bytes,2,opt,name=mode,proto3" json:"mode"`
	Day              string             `protobuf:"bytes,3,opt,name=day,proto3" json:"day"`
	Rules            string             `protobuf:"bytes,4,opt,name=rules,proto3" json:"rules"`
	Phase            string             `protobuf:"bytes,5,opt,name=phase,proto3" json:"phase"`
	StartMS          int64              `protobuf:"varint,6,opt,name=start_ms,proto3" json:"start_ms"`
	EndMS            int64              `protobuf:"varint,7,opt,name=end_ms,proto3" json:"end_ms"`
	Score            int32              `protobuf:"varint,8,opt,name=score,proto3" json:"score"`
	Pairs            int32              `protobuf:"varint,9,opt,name=pairs,proto3" json:"pairs"`
	Boards           int32              `protobuf:"varint,10,opt,name=boards,proto3" json:"boards"`
	Multiplier       int32              `protobuf:"varint,11,opt,name=multiplier,proto3" json:"multiplier"`
	ComboUntilMS     int64              `protobuf:"varint,12,opt,name=combo_until_ms,proto3" json:"combo_until_ms"`
	Board            int32              `protobuf:"varint,13,opt,name=board,proto3" json:"board"`
	Items            []*ChallengeItem   `protobuf:"bytes,14,rep,name=items,proto3" json:"items"`
	Members          []*ChallengeMember `protobuf:"bytes,15,rep,name=members,proto3" json:"members"`
	EndReason        string             `protobuf:"bytes,16,opt,name=end_reason,proto3" json:"end_reason"`
	RemainingMS      int64              `protobuf:"varint,17,opt,name=remaining_ms,proto3" json:"remaining_ms"`
	ComboRemainingMS int64              `protobuf:"varint,18,opt,name=combo_remaining_ms,proto3" json:"combo_remaining_ms"`
}

func (m *Challenge) Reset()         { *m = Challenge{} }
func (m *Challenge) String() string { return proto.CompactTextString(m) }
func (*Challenge) ProtoMessage()    {}

// ChallengeItem 的坐标是原位；Place 只允许 ground、hand、tray。
type ChallengeItem struct {
	ID        string  `protobuf:"bytes,1,opt,name=id,proto3" json:"id"`
	Kind      string  `protobuf:"bytes,2,opt,name=kind,proto3" json:"kind"`
	X         float64 `protobuf:"fixed64,3,opt,name=x,proto3" json:"x"`
	Y         float64 `protobuf:"fixed64,4,opt,name=y,proto3" json:"y"`
	Z         float64 `protobuf:"fixed64,5,opt,name=z,proto3" json:"z"`
	BlockerID string  `protobuf:"bytes,6,opt,name=blocker_id,proto3" json:"blocker_id"`
	Place     string  `protobuf:"bytes,7,opt,name=place,proto3" json:"place"`
	OwnerID   string  `protobuf:"bytes,8,opt,name=owner_id,proto3" json:"owner_id"`
	Slot      int32   `protobuf:"varint,9,opt,name=slot,proto3" json:"slot"`
	Blocked   bool    `protobuf:"varint,10,opt,name=blocked,proto3" json:"blocked"`
}

func (m *ChallengeItem) Reset()         { *m = ChallengeItem{} }
func (m *ChallengeItem) String() string { return proto.CompactTextString(m) }
func (*ChallengeItem) ProtoMessage()    {}

// ChallengeMember 仅含局内公开身份与配对贡献，不含凭据。
type ChallengeMember struct {
	ID    string `protobuf:"bytes,1,opt,name=id,proto3" json:"id"`
	Name  string `protobuf:"bytes,2,opt,name=name,proto3" json:"name"`
	Pairs int32  `protobuf:"varint,3,opt,name=pairs,proto3" json:"pairs"`
}

func (m *ChallengeMember) Reset()         { *m = ChallengeMember{} }
func (m *ChallengeMember) String() string { return proto.CompactTextString(m) }
func (*ChallengeMember) ProtoMessage()    {}
