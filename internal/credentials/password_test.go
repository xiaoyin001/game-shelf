// 文件说明：覆盖规范名、密码边界、随机盐和受限哈希解码，不用快速假哈希代替真实算法。
package credentials

import (
	"strings"
	"testing"
)

func TestAccountNormalization(t *testing.T) {
	for _, v := range []string{"  Ａlice  ", "ALICE", "Alice"} {
		key, _, err := Normalize(v)
		if err != nil || key != "alice" {
			t.Fatal(key, err)
		}
	}
	if key, _, err := Normalize("林间旅人"); err != nil || key != "林间旅人" {
		t.Fatal(key, err)
	}
	for _, v := range []string{"", "a", "a b", "<script>", "a/b", strings.Repeat("a", 25)} {
		if _, _, err := Normalize(v); err == nil {
			t.Fatalf("accepted %q", v)
		}
	}
}
func TestPasswordHash(t *testing.T) {
	password := "strong-test-passphrase-91"
	a, err := Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	if a == b || strings.Contains(a, password) || !Verify(a, password) || Verify(a, "wrong-test-passphrase-91") {
		t.Fatal("hash verification or salt failed")
	}
	for _, v := range []string{"", "plaintext", strings.Replace(a, "m=19456", "m=999999999", 1), a + "extra", strings.Replace(a, "v=19", "v=18", 1)} {
		if Verify(v, password) {
			t.Fatal("accepted malformed hash")
		}
	}
}
func TestPasswordBoundaries(t *testing.T) {
	for _, v := range []string{"short", strings.Repeat(" ", 12), strings.Repeat("a", 129), "twelve-test\x00end"} {
		if ValidatePassword(v) == nil {
			t.Fatal("accepted invalid password")
		}
	}
	for _, v := range []string{"  valid-password  ", strings.Repeat("好", 12), strings.Repeat("a", 128)} {
		if ValidatePassword(v) != nil {
			t.Fatal("rejected valid password")
		}
	}
}
