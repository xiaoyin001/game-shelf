// 文件说明：验证密码工作队列的并发/取消边界和哈希的默认 JSON 隔离，不冒充真实存储测试。
package credentials

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestHashWorkersBoundedAndCancelable(t *testing.T) {
	s := &Store{workers: make(chan struct{}, 2)}
	a, err := s.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.acquire(context.Background()); !errors.Is(err, ErrBusy) {
		t.Fatal("unbounded hashing")
	}
	a()
	b()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = s.acquire(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("cancel ignored")
	}
	release, err := s.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	release()
}
func TestInvalidPasswordRejectedBeforeStorage(t *testing.T) {
	s := &Store{}
	for _, password := range []string{"12345", strings.Repeat("a", 33), "abc中文def", "abc def"} {
		if _, err := s.Register(context.Background(), "test-user", password); !errors.Is(err, ErrInvalidPassword) {
			t.Fatal("register reached storage with invalid password")
		}
		if _, err := s.Login(context.Background(), "test-user", password); !errors.Is(err, ErrInvalidPassword) {
			t.Fatal("login reached storage with invalid password")
		}
	}
}

func TestAccountHashNeverJSON(t *testing.T) {
	data, _ := json.Marshal(Account{PasswordHash: "private-hash"})
	if strings.Contains(string(data), "private-hash") {
		t.Fatal("hash leaked")
	}
}
