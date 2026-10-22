// 文件说明：以真实 HTTP、MongoDB、Redis 和在线网关验证凭据、存储与错误登录不顶号。
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"game-shelf/internal/credentials"
	"game-shelf/pkg/contracts"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

const testPassword = "Testing-passphrase-93!"

type accountResult struct {
	status int
	body   string
	reply  contracts.LoginHTTPReply
}

func accountRequest(t *testing.T, path, account, password, channel string) accountResult {
	t.Helper()
	data, _ := json.Marshal(contracts.LoginHTTPRequest{Account: account, Password: password, Channel: channel})
	res, err := (&http.Client{Timeout: 6 * time.Second}).Post("http://127.0.0.1:11000"+path, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var reply contracts.LoginHTTPReply
	_ = json.Unmarshal(raw, &reply)
	return accountResult{status: res.StatusCode, body: string(raw), reply: reply}
}
func TestLivePasswordValidationAndPersistence(t *testing.T) {
	live(t)
	account := name()
	first := accountRequest(t, "/register", account, testPassword, "browser")
	if first.status != 201 || first.reply.UserID == "" || first.reply.Token == "" {
		t.Fatal(first.status, first.body)
	}
	duplicate := accountRequest(t, "/register", strings.ToUpper(account), testPassword, "other")
	if duplicate.status != 409 || duplicate.reply.Token != "" {
		t.Fatal("duplicate accepted", duplicate)
	}
	missing := accountRequest(t, "/login", account, "", "browser")
	if missing.status != 400 || missing.reply.Token != "" {
		t.Fatal("name-only accepted")
	}
	bad := accountRequest(t, "/login", account, "Wrong-passphrase-33!", "browser")
	unknown := accountRequest(t, "/login", name(), "Wrong-passphrase-33!", "browser")
	if bad.status != 401 || unknown.status != 401 || bad.body != unknown.body || bad.reply.Token != "" {
		t.Fatal("password or account enumeration", bad.status, unknown.status)
	}
	second := accountRequest(t, "/login", strings.ToUpper(account), testPassword, "another-channel")
	if second.status != 200 || second.reply.UserID != first.reply.UserID || second.reply.Token == first.reply.Token {
		t.Fatal("unstable authenticated identity")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	db, err := mongo.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:27017"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Disconnect(ctx)
	var saved credentials.Account
	if err = db.Database("game_shelf").Collection("accounts").FindOne(ctx, bson.M{"user_id": first.reply.UserID}).Decode(&saved); err != nil {
		t.Fatal(err)
	}
	if saved.PasswordHash == testPassword || !strings.HasPrefix(saved.PasswordHash, "$argon2id$") || !credentials.Verify(saved.PasswordHash, testPassword) {
		t.Fatal("unsafe stored verifier")
	}
	if strings.Contains(first.body, "password") || strings.Contains(first.body, saved.PasswordHash) {
		t.Fatal("secret in reply")
	}
}
func TestLiveFailedPasswordCannotReplaceSession(t *testing.T) {
	live(t)
	account := name()
	first := login(t, account)
	current := auth(t, first)
	bad := accountRequest(t, "/login", account, "Wrong-passphrase-33!", "browser")
	if bad.status != 401 {
		t.Fatal(bad.status)
	}
	send(t, current, "ping", nil)
	// 网关 ping 不返回 pong，持续收到后续权威快照才能证明旧会话仍有效。
	for range 3 {
		state := read(t, current, "state").State
		found := false
		for _, player := range state.Players {
			if player.ID == first.UserID {
				found = true
			}
		}
		if !found {
			t.Fatal("failed login removed current player")
		}
	}
	next := accountRequest(t, "/login", account, testPassword, "changed-channel")
	if next.status != 200 {
		t.Fatal(next.status)
	}
	_ = auth(t, next.reply)
	closed(t, current)
}
func TestLiveConcurrentRegistration(t *testing.T) {
	live(t)
	account := name()
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses <- accountRequest(t, "/register", account, testPassword, "browser").status
		}()
	}
	wg.Wait()
	close(statuses)
	created, duplicate := 0, 0
	for status := range statuses {
		if status == 201 {
			created++
		}
		if status == 409 {
			duplicate++
		}
	}
	if created != 1 || duplicate != 1 {
		t.Fatal("registration not unique", created, duplicate)
	}
}
func TestLiveLegacyPlayersAreNotClaimed(t *testing.T) {
	live(t)
	account := name()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	db, err := mongo.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:27017"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Disconnect(ctx)
	old := bson.M{"user_id": "legacy-" + account, "account": account, "channel": "browser", "legacy_marker": true}
	if _, err = db.Database("game_shelf").Collection("players").InsertOne(ctx, old); err != nil {
		t.Fatal(err)
	}
	fresh := accountRequest(t, "/register", account, testPassword, "browser")
	if fresh.status != 201 || fresh.reply.UserID == old["user_id"] {
		t.Fatal("legacy identity claimed")
	}
	var kept bson.M
	if err = db.Database("game_shelf").Collection("players").FindOne(ctx, bson.M{"user_id": old["user_id"]}).Decode(&kept); err != nil || kept["legacy_marker"] != true {
		t.Fatal("legacy record modified", err)
	}
}
func TestLivePasswordAttemptLimit(t *testing.T) {
	live(t)
	account := name()
	if r := accountRequest(t, "/register", account, testPassword, "browser"); r.status != 201 {
		t.Fatal(r.status)
	}
	for range 11 {
		r := accountRequest(t, "/login", account, "Wrong-passphrase-33!", "browser")
		if r.status != 401 {
			t.Fatal("premature limit", r.status)
		}
	}
	if r := accountRequest(t, "/login", account, testPassword, "browser"); r.status != 429 || r.reply.Token != "" {
		t.Fatal("limit bypass", r.status)
	}
}
