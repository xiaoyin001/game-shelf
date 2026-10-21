// 文件说明：校验当前在线状态 protobuf 契约能够完整往返。
package contracts

import (
	"github.com/golang/protobuf/proto"
	"testing"
)

func TestPresenceProtobufRoundTrip(t *testing.T) {
	source := &WorldResp{State: &WorldState{TimeMS: 42, Width: 1200, Height: 800, Players: []*WorldPlayer{{ID: "a", Name: "旅人", X: 560, Y: 400}}}}
	data, err := proto.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	var target WorldResp
	if err := proto.Unmarshal(data, &target); err != nil || !proto.Equal(source, &target) {
		t.Fatal("world round trip failed", err)
	}
}
