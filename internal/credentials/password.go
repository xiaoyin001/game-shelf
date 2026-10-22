// 文件说明：校验账号输入并用固定、受限成本的 Argon2id 格式保存与验证密码。
package credentials

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

var (
	ErrInvalidAccount     = errors.New("invalid_account")
	ErrInvalidPassword    = errors.New("invalid_password")
	ErrInvalidCredentials = errors.New("invalid_credentials")
	ErrAccountExists      = errors.New("account_exists")
	ErrBusy               = errors.New("auth_busy")
)

const parameters = "m=19456,t=2,p=1"

// Normalize 保留首次注册的显示名，以规范化折叠后的键约束唯一身份。
func Normalize(account string) (key, display string, err error) {
	display = norm.NFKC.String(strings.TrimSpace(account))
	if !utf8.ValidString(display) || utf8.RuneCountInString(display) < 2 || utf8.RuneCountInString(display) > 24 {
		return "", "", ErrInvalidAccount
	}
	for _, r := range display {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
			return "", "", ErrInvalidAccount
		}
	}
	return cases.Fold().String(display), display, nil
}

// ValidatePassword 只接受 6–32 位 ASCII 可见字符，不截断、清理或规范化口令。
func ValidatePassword(password string) error {
	if len(password) < 6 || len(password) > 32 {
		return ErrInvalidPassword
	}
	for i := range len(password) {
		if password[i] < 0x21 || password[i] > 0x7e {
			return ErrInvalidPassword
		}
	}
	return nil
}

// Hash 每次生成新盐，输出可验证版本和成本的 PHC 格式。
func Hash(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := argon2.IDKey([]byte(password), salt, 2, 19456, 1, 32)
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=19$%s$%s$%s", parameters, enc.EncodeToString(salt), enc.EncodeToString(sum)), nil
}

// Verify 先限定格式和成本再派生密钥，不接受可导致任意内存分配的存储参数。
func Verify(encoded, password string) bool {
	if ValidatePassword(password) != nil {
		return false
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" || parts[3] != parameters || len(parts[4]) != 22 || len(parts[5]) != 43 {
		return false
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		return false
	}
	expected, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil || len(expected) != 32 {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, 2, 19456, 1, 32)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
