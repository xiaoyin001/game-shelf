// 文件说明：对真实四服务组合验证 HTTP 登录、MongoDB 账号保存及原 TCP 大厅行为。
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"game-shelf/internal/protocol"
	"game-shelf/pkg/contracts"
	"github.com/golang/protobuf/proto"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"
)

func live(t *testing.T) {
	t.Helper()
	if os.Getenv("GAME_SHELF_INTEGRATION") != "1" {
		t.Skip("执行 npm run test:services 启动真实服务")
	}
}
func name() string { return "test-" + strconv.FormatInt(time.Now().UnixNano(), 36) }
func login(t *testing.T, account string) contracts.LoginHTTPReply {
	t.Helper()
	registration := accountRequest(t, "/register", account, testPassword, "integration")
	if registration.status != http.StatusCreated && registration.status != http.StatusConflict {
		t.Fatalf("register status: %d %s", registration.status, registration.body)
	}
	data, _ := json.Marshal(contracts.LoginHTTPRequest{Account: account, Password: testPassword, Channel: "integration"})
	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Post("http://127.0.0.1:11000/login", "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("login %d: %s", res.StatusCode, body)
	}
	var reply contracts.LoginHTTPReply
	if err := json.NewDecoder(res.Body).Decode(&reply); err != nil {
		t.Fatal(err)
	}
	return reply
}
func TestLiveHTTPAndAccountPersistence(t *testing.T) {
	live(t)
	account := name()
	first := login(t, account)
	second := login(t, account)
	if first.UserID == "" || first.UserID != second.UserID || first.Token == second.Token {
		t.Fatal("unstable account or reused token")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	db, err := mongo.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:27017"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Disconnect(ctx)
	count, err := db.Database("game_shelf").Collection("accounts").CountDocuments(ctx, bson.M{"user_id": first.UserID})
	if err != nil || count != 1 {
		t.Fatal("login did not persist exactly one player", count, err)
	}
}
func TestLiveOriginalTCP(t *testing.T) {
	live(t)
	reply := login(t, name())
	conn, err := net.DialTimeout("tcp", reply.GateAddr, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	call := func(id uint32, body proto.Message, want uint32) protocol.Message {
		t.Helper()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		msg, err := protocol.MarshalMessage(id, 1, body)
		if err != nil {
			t.Fatal(err)
		}
		if err := protocol.WriteMessage(conn, msg); err != nil {
			t.Fatal(err)
		}
		result, err := protocol.ReadMessage(conn, 1048576)
		if err != nil || result.ProtoID != want {
			t.Fatal("unexpected TCP response", err, result.ProtoID)
		}
		return result
	}
	result := call(contracts.CS_GateAuthReq, &contracts.GateAuthReq{Token: reply.Token}, contracts.SC_GateAuthResp)
	var auth contracts.GateAuthResp
	if err := protocol.UnmarshalBody(result, &auth); err != nil || auth.Code != 0 || auth.UserID != reply.UserID {
		t.Fatal("auth failed", err)
	}
	call(contracts.CS_PingReq, &contracts.PingReq{}, contracts.SC_PingResp)
	result = call(contracts.CS_LobbyListReq, &contracts.LobbyListReq{UserID: reply.UserID}, contracts.SC_LobbyListResp)
	var lobby contracts.LobbyListResp
	if err := protocol.UnmarshalBody(result, &lobby); err != nil || len(lobby.Rooms) != 2 {
		t.Fatal("lobby failed", err)
	}
}
