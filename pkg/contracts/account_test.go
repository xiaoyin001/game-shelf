// 文件说明：验证凭据内部 protobuf 往返且日志/JSON 不输出密码，公共回复不含秘密。
package contracts

import (
	"encoding/json"
	"github.com/golang/protobuf/proto"
	"strings"
	"testing"
)

func TestAccountRequestSecretBoundary(t *testing.T) {
	request := &AccountRequest{Account: "traveller", Password: "private-passphrase-93"}
	data, err := proto.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded AccountRequest
	if err = proto.Unmarshal(data, &decoded); err != nil || decoded.Password != request.Password {
		t.Fatal("protobuf credentials lost", err)
	}
	output, _ := json.Marshal(request)
	if strings.Contains(string(output), request.Password) || strings.Contains(request.String(), request.Password) {
		t.Fatal("password exposed")
	}
}
