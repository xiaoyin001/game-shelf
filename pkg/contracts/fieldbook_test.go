// 文件说明：覆盖收藏册的protobuf标量/嵌套往返，避免零值和presence回归。
package contracts

import (
	"github.com/golang/protobuf/proto"
	"reflect"
	"testing"
)

func TestFieldBookProtobuf(t *testing.T) {
	messages := []proto.Message{
		&BookRequest{UserID: "account", Journal: &Journal{Revision: 3, Found: []string{"bell"}, Active: "letter", Step: 1}},
		&WorldResp{Book: &FieldBook{Collections: []*Collection{{ID: "bell", Found: true, Story: "故事"}, {ID: "letter"}}, Trail: &Trail{ID: "letter", X: 790, Y: 460, Z: 0, Step: 1, Total: 2}}},
	}
	for _, original := range messages {
		wire, err := proto.Marshal(original)
		if err != nil {
			t.Fatal(err)
		}
		decoded := reflect.New(reflect.TypeOf(original).Elem()).Interface().(proto.Message)
		if err = proto.Unmarshal(wire, decoded); err != nil || !proto.Equal(original, decoded) {
			t.Fatalf("roundtrip: %v, %+v", err, decoded)
		}
	}
}
