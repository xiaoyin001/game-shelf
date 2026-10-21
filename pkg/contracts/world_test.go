// 文件说明：验证嵌套世界快照通过现有 protobuf 编解码后保持完整。
package contracts

import (
	"github.com/golang/protobuf/proto"
	"testing"
)

func TestWorldProtobufRoundTrip(t *testing.T) {
	input := &WorldResp{State: &WorldState{TimeMS: 42, Width: 1200, Height: 800,
		Players: []*WorldPlayer{{ID: "a", Name: "旅人", X: 560.5, Y: 400, Score: 2}},
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
	if !proto.Equal(input, &output) {
		t.Fatalf("round trip mismatch: %v", &output)
	}
}
