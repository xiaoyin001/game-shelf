// 文件说明：定义仅由lobby提交的完成成绩、公开榜单查询及私有保存状态。
package contracts

import "github.com/golang/protobuf/proto"

const (
	LBDB_SaveScoreReq    uint32 = 600007
	DBLB_SaveScoreResp   uint32 = 600008
	LBDB_QueryScoresReq  uint32 = 600009
	DBLB_QueryScoresResp uint32 = 600010
)

// ScoreRecord 是正常完成且资格确认后的不可变回合，不包含手持、凭据或客户端时钟。
type ScoreRecord struct {
	RoundID string             `protobuf:"bytes,1,opt,name=round_id,proto3" json:"round_id" bson:"round_id"`
	Mode    string             `protobuf:"bytes,2,opt,name=mode,proto3" json:"mode" bson:"mode"`
	Day     string             `protobuf:"bytes,3,opt,name=day,proto3" json:"day" bson:"day"`
	Rules   string             `protobuf:"bytes,4,opt,name=rules,proto3" json:"rules" bson:"rules"`
	StartMS int64              `protobuf:"varint,5,opt,name=start_ms,proto3" json:"start_ms" bson:"start_ms"`
	EndMS   int64              `protobuf:"varint,6,opt,name=end_ms,proto3" json:"end_ms" bson:"end_ms"`
	Score   int32              `protobuf:"varint,7,opt,name=score,proto3" json:"score" bson:"score"`
	Pairs   int32              `protobuf:"varint,8,opt,name=pairs,proto3" json:"pairs" bson:"pairs"`
	Boards  int32              `protobuf:"varint,9,opt,name=boards,proto3" json:"boards" bson:"boards"`
	Members []*ChallengeMember `protobuf:"bytes,10,rep,name=members,proto3" json:"members" bson:"members"`
}

func (m *ScoreRecord) Reset()         { *m = ScoreRecord{} }
func (m *ScoreRecord) String() string { return proto.CompactTextString(m) }
func (*ScoreRecord) ProtoMessage()    {}

// ScoresRequest 的UserID由gate认证后填入，Record只存在于lobby到db的内部写请求。
type ScoresRequest struct {
	UserID string       `protobuf:"bytes,1,opt,name=user_id,proto3" json:"user_id"`
	Mode   string       `protobuf:"bytes,2,opt,name=mode,proto3" json:"mode"`
	Day    string       `protobuf:"bytes,3,opt,name=day,proto3" json:"day"`
	Record *ScoreRecord `protobuf:"bytes,4,opt,name=record,proto3" json:"record,omitempty"`
}

func (m *ScoresRequest) Reset()         { *m = ScoresRequest{} }
func (m *ScoresRequest) String() string { return proto.CompactTextString(m) }
func (*ScoresRequest) ProtoMessage()    {}

type ScoresReply struct {
	Error string       `protobuf:"bytes,1,opt,name=error,proto3" json:"error,omitempty"`
	Board *Leaderboard `protobuf:"bytes,2,opt,name=board,proto3" json:"board,omitempty"`
}

func (m *ScoresReply) Reset()         { *m = ScoresReply{} }
func (m *ScoresReply) String() string { return proto.CompactTextString(m) }
func (*ScoresReply) ProtoMessage()    {}

// Leaderboard 的Entries已按名次排序，Best只属于当前已认证账号，均从已确认记录派生。
type Leaderboard struct {
	Mode     string         `protobuf:"bytes,1,opt,name=mode,proto3" json:"mode"`
	Day      string         `protobuf:"bytes,2,opt,name=day,proto3" json:"day"`
	Rules    string         `protobuf:"bytes,3,opt,name=rules,proto3" json:"rules"`
	Entries  []*ScoreRecord `protobuf:"bytes,4,rep,name=entries,proto3" json:"entries"`
	BestSolo *ScoreRecord   `protobuf:"bytes,5,opt,name=best_solo,proto3" json:"best_solo,omitempty"`
	BestDuo  *ScoreRecord   `protobuf:"bytes,6,opt,name=best_duo,proto3" json:"best_duo,omitempty"`
	Error    string         `protobuf:"bytes,7,opt,name=error,proto3" json:"error,omitempty"`
}

func (m *Leaderboard) Reset()         { *m = Leaderboard{} }
func (m *Leaderboard) String() string { return proto.CompactTextString(m) }
func (*Leaderboard) ProtoMessage()    {}

// Settlement 状态由后台真实提交产生，saved前不承诺持久化。
type Settlement struct {
	RoundID string `protobuf:"bytes,1,opt,name=round_id,proto3" json:"round_id"`
	Status  string `protobuf:"bytes,2,opt,name=status,proto3" json:"status"`
}

func (m *Settlement) Reset()         { *m = Settlement{} }
func (m *Settlement) String() string { return proto.CompactTextString(m) }
func (*Settlement) ProtoMessage()    {}
