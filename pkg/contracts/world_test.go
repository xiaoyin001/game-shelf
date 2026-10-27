// 文件说明：验证当前世界/配对消息的 protobuf 标量零值与嵌套往返，停用旧字段号不复用。
package contracts

import (
	"github.com/golang/protobuf/proto"
	"testing"
)

func TestProto3ScalarPresenceAndZeroEncoding(t *testing.T) {
	for _, message := range []proto.Message{&WorldReq{}, &WorldResp{}, &WorldPlayer{}, &Challenge{}, &ChallengeMember{}, &ChallengeItem{}, &ChallengeRoom{}, &RoomMember{}} {
		descriptor := proto.MessageReflect(message).Descriptor()
		t.Run(string(descriptor.Name()), func(t *testing.T) {
			for i := 0; i < descriptor.Fields().Len(); i++ {
				f := descriptor.Fields().Get(i)
				if f.Message() == nil && !f.IsList() && !f.IsMap() && f.HasPresence() {
					t.Fatal("proto3 pointer presence", f.FullName())
				}
			}
			data, err := proto.Marshal(message)
			if err != nil || len(data) != 0 {
				t.Fatal("defaults not omitted", data, err)
			}
		})
	}
}
func TestWorldProtobufRoundTrip(t *testing.T) {
	input := &WorldResp{State: &WorldState{TimeMS: 42, Width: 1200, Height: 800, SceneVersion: "mosslight-pair-1",
		Players: []*WorldPlayer{{ID: "a", Name: "旅人", X: 560.5, Y: 400, Z: 42.5, Grounded: true}},
		Nodes:   []*WorldNode{{ID: "n", X: 620}}, Chats: []*WorldChat{{ID: 1, Name: "旅人", Text: "你好"}},
	}, Challenge: &Challenge{ID: "round", Phase: "running", Score: 100, Multiplier: 15, StartMS: 42, EndMS: 180042,
		Items:   []*ChallengeItem{{ID: "1-a", Kind: "leaf", Place: "hand", OwnerID: "a", Blocked: true}},
		Members: []*ChallengeMember{{ID: "a", Name: "旅人", Pairs: 1}},
	}}
	data, err := proto.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var output WorldResp
	if err := proto.Unmarshal(data, &output); err != nil || !proto.Equal(input, &output) {
		t.Fatal("state mismatch", err, &output)
	}
	req := &WorldReq{Action: "set_ready", ItemID: "1-a", RoundID: "round", RoomID: "room", RoomCode: "ABC234", Ready: true}
	data, err = proto.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var decoded WorldReq
	if err := proto.Unmarshal(data, &decoded); err != nil || !proto.Equal(req, &decoded) {
		t.Fatal("intent mismatch", err)
	}
	descriptor := proto.MessageReflect(&WorldReq{}).Descriptor()
	for _, n := range []int32{8, 9, 10, 11} {
		for i := 0; i < descriptor.Fields().Len(); i++ {
			if int32(descriptor.Fields().Get(i).Number()) == n {
				t.Fatal("retired field reused", n)
			}
		}
	}
}
