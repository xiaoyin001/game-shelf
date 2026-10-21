// 文件说明：验证聊天长度、频率限制与有界历史。
package world

import (
	"fmt"
	"game-shelf/pkg/contracts"
	"strings"
	"testing"
	"time"
)

func TestChatValidationAndBoundedHistory(t *testing.T) {
	now := time.Unix(1000, 0)
	w := New()
	enter(w, "a", "s", now)
	send := func(value string, at time.Time) *contracts.WorldResp {
		return w.Apply(&contracts.WorldReq{UserID: "a", SessionID: "s", Action: "chat", Text: value}, at)
	}
	if send(strings.Repeat("字", 141), now).Error != "invalid_chat" {
		t.Fatal("long chat accepted")
	}
	if send("<script>alert(1)</script>", now).Error != "" {
		t.Fatal("plain text not retained")
	}
	if send("again", now).Error != "chat_too_fast" {
		t.Fatal("spam accepted")
	}
	for i := 1; i <= 40; i++ {
		if send(fmt.Sprint(i), now.Add(time.Duration(i)*time.Second)).Error != "" {
			t.Fatal("valid chat rejected")
		}
	}
	r := command(w, "a", "s", "poll", now.Add(40*time.Second))
	if len(r.State.Chats) != 30 {
		t.Fatal("unbounded history")
	}
}
