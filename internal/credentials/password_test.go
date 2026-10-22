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
	for _, v := range []string{"", "12345", strings.Repeat("a", 33), strings.Repeat("a", 128), "中文密码测试", "abc中文def", "abc😀def", "Ａbc123", "abc def", " abc123", "abc123 ", "abc\tdef", "abc\ndef", "abc\rdef", "abc\x00def", "abc\x7fdef", "abc\u200bdef", "abc\u00a0def"} {
		if ValidatePassword(v) == nil {
			t.Fatal("accepted invalid password")
		}
		if _, err := Hash(v); err != ErrInvalidPassword {
			t.Fatal("hash accepted invalid input")
		}
	}
	for _, v := range []string{"123456", "abcdef", "!@#$%^", strings.Repeat("a", 32), "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"} {
		hash, err := Hash(v)
		if err != nil || !Verify(hash, v) || Verify(hash, v+"中") {
			t.Fatal("valid boundary did not round trip")
		}
	}
}

func TestPasswordCharacterRange(t *testing.T) {
	for b := 0; b < 256; b++ {
		err := ValidatePassword("abcde" + string([]byte{byte(b)}))
		if (err == nil) != (b >= 0x21 && b <= 0x7e) {
			t.Fatalf("incorrect byte boundary: %d", b)
		}
	}
}
