// 文件说明：通过真实浏览器协议验证凭据、Origin、消息边界及会话清理保护。
package integration

import (
	"context"
	"encoding/json"
	"game-shelf/internal/config"
	"game-shelf/internal/redisx"
	"game-shelf/internal/session"
	"game-shelf/pkg/contracts"
	"github.com/coder/websocket"
	"net/http"
	"strings"
	"testing"
	"time"
)

type event struct {
	Version int                    `json:"version"`
	Type    string                 `json:"type"`
	UserID  string                 `json:"user_id"`
	Error   string                 `json:"error"`
	State   *contracts.WorldState  `json:"state"`
	Quest   *contracts.WorldQuest  `json:"quest"`
	Dialog  *contracts.WorldDialog `json:"dialog"`
}

func dial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"http://127.0.0.1:11000"}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	return conn
}
func send(t *testing.T, conn *websocket.Conn, kind string, fields map[string]any) {
	t.Helper()
	if fields == nil {
		fields = map[string]any{}
	}
	fields["version"] = 1
	fields["type"] = kind
	data, _ := json.Marshal(fields)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatal(err)
	}
}
func read(t *testing.T, conn *websocket.Conn, want string) event {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var e event
		if err := json.Unmarshal(data, &e); err != nil {
			t.Fatal(err)
		}
		if e.Version != 1 {
			t.Fatal("missing version")
		}
		if e.Type == want {
			return e
		}
		if e.Type == "error" {
			t.Fatalf("unexpected error: %s", data)
		}
	}
}
func closed(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	for {
		_, _, err := conn.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				t.Fatal("server did not close connection")
			}
			return
		}
	}
}
func auth(t *testing.T, reply contracts.LoginHTTPReply) *websocket.Conn {
	t.Helper()
	conn := dial(t, reply.GateWebSocketURL)
	send(t, conn, "auth", map[string]any{"token": reply.Token})
	read(t, conn, "ready")
	return conn
}

func TestLiveBrowserPresenceAndReplacement(t *testing.T) {
	live(t)
	account := name()
	first := login(t, account)
	old := auth(t, first)
	other := login(t, name())
	peer := auth(t, other)
	state := read(t, peer, "state").State
	found := false
	for _, p := range state.Players {
		if p.ID == first.UserID {
			found = true
		}
	}
	if !found {
		t.Fatal("not in shared world")
	}
	second := login(t, account)
	fresh := auth(t, second)
	closed(t, old)
	for range 4 {
		state = read(t, fresh, "state").State
	}
	found = false
	for _, p := range state.Players {
		if p.ID == first.UserID {
			found = true
		}
	}
	if !found {
		t.Fatal("old cleanup removed new player")
	}
	replay := dial(t, second.GateWebSocketURL)
	send(t, replay, "auth", map[string]any{"token": second.Token})
	if read(t, replay, "error").Error != "invalid_or_expired_token" {
		t.Fatal("token replay accepted")
	}
}
func TestLiveInvalidAuthentication(t *testing.T) {
	live(t)
	t.Run("invalid token", func(t *testing.T) {
		reply := login(t, name())
		conn := dial(t, reply.GateWebSocketURL)
		send(t, conn, "auth", map[string]any{"token": "invalid"})
		if read(t, conn, "error").Error != "invalid_or_expired_token" {
			t.Fatal("invalid token accepted")
		}
	})
	t.Run("auth required", func(t *testing.T) {
		reply := login(t, name())
		conn := dial(t, reply.GateWebSocketURL)
		send(t, conn, "input", nil)
		if read(t, conn, "error").Error != "auth_required" {
			t.Fatal("anonymous input accepted")
		}
	})
	t.Run("origin", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		conn, resp, err := websocket.Dial(ctx, "ws://127.0.0.1:12001/ws", &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"https://untrusted.invalid"}}})
		if conn != nil {
			conn.CloseNow()
		}
		if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
			t.Fatalf("origin accepted: %v %v", resp, err)
		}
	})
	t.Run("expired token", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		redis, err := redisx.Connect(ctx, config.Redis{Addr: "127.0.0.1:6379"})
		if err != nil {
			t.Fatal(err)
		}
		defer redis.Close()
		token, err := session.NewStore(redis).CreateLoginToken(ctx, name(), "expired", "integration", "gate-local-1", 20*time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(50 * time.Millisecond)
		conn := dial(t, "ws://127.0.0.1:12001/ws")
		send(t, conn, "auth", map[string]any{"token": token.Token})
		if read(t, conn, "error").Error != "invalid_or_expired_token" {
			t.Fatal("expired token accepted")
		}
	})
}
func TestLiveRejectsMalformedInput(t *testing.T) {
	live(t)
	for _, tc := range []struct {
		name string
		kind websocket.MessageType
		data string
	}{
		{"spoof identity", websocket.MessageText, `{"version":1,"type":"ping","user_id":"another-user"}`},
		{"unsupported version", websocket.MessageText, `{"version":2,"type":"ping"}`},
		{"oversized packet", websocket.MessageText, strings.Repeat("x", 5000)},
		{"binary packet", websocket.MessageBinary, "x"},
		{"trailing json", websocket.MessageText, `{"version":1,"type":"ping"}{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn := auth(t, login(t, name()))
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := conn.Write(ctx, tc.kind, []byte(tc.data)); err != nil {
				t.Fatal(err)
			}
			closed(t, conn)
		})
	}
	t.Run("rate limit", func(t *testing.T) {
		conn := auth(t, login(t, name()))
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		for range 40 {
			if err := conn.Write(ctx, websocket.MessageText, []byte(`{"version":1,"type":"ping"}`)); err != nil {
				break
			}
		}
		closed(t, conn)
	})
}
