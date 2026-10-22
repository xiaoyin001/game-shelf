// 文件说明：验证嵌套世界快照通过现有 protobuf 编解码后保持完整。
package contracts

import (
	"github.com/golang/protobuf/proto"
	"testing"
)

func TestProto3ScalarPresenceAndZeroEncoding(t *testing.T) {
	for _, message := range []proto.Message{&WorldReq{}, &WorldResp{}, &WorldPlayer{}, &WorldQuest{}, &WorldDialog{}, &DialogChoice{}} {
		descriptor := proto.MessageReflect(message).Descriptor()
		t.Run(string(descriptor.Name()), func(t *testing.T) {
			for i := 0; i < descriptor.Fields().Len(); i++ {
				field := descriptor.Fields().Get(i)
				if field.Message() == nil && !field.IsList() && !field.IsMap() && field.HasPresence() {
					t.Fatal("proto3 scalar incorrectly has pointer presence", field.FullName())
				}
			}
			data, err := proto.Marshal(message)
			if err != nil || len(data) != 0 {
				t.Fatal("proto3 defaults were not omitted", data, err)
			}
		})
	}
}

func TestWorldProtobufRoundTrip(t *testing.T) {
	input := &WorldResp{Quest: &WorldQuest{Status: "active", Progress: 1, Goal: 2, Reward: 3},
		Dialog: &WorldDialog{NPCID: "keeper", Name: "阿苔", Text: "欢迎", Choices: []*DialogChoice{{ID: "accept", Label: "接取"}}}, State: &WorldState{TimeMS: 42, Width: 1200, Height: 800, SceneVersion: "mosslight-physics-1",
			Players: []*WorldPlayer{{ID: "a", Name: "旅人", X: 560.5, Y: 400, Z: 42.5, Grounded: true, Score: 2}},
			Nodes:   []*WorldNode{{ID: "n", X: 620, ReadyAtMS: 100}},
			Chats:   []*WorldChat{{ID: 1, Name: "旅人", Text: "你好"}},
		}}
	data, err := proto.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var output WorldResp
	if err := proto.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	req := &WorldReq{Action: "choose", NPCID: "keeper", ChoiceID: "complete"}
	data, err = proto.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var decoded WorldReq
	if err := proto.Unmarshal(data, &decoded); err != nil || !proto.Equal(req, &decoded) {
		t.Fatal("NPC request mismatch", err)
	}
	if !proto.Equal(input, &output) {
		t.Fatalf("round trip mismatch: %v", &output)
	}
}
