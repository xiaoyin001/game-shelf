// 文件说明：以 MongoDB 原子唯一账号文档保存凭据，保留旧试玩资料并限制慢哈希并发。
package credentials

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Account 的哈希不进入 HTTP 或内部认证回复。
type Account struct {
	Key          string    `bson:"_id"`
	UserID       string    `bson:"user_id"`
	Name         string    `bson:"account"`
	PasswordHash string    `bson:"password_hash" json:"-"`
	CreatedAt    time.Time `bson:"created_at"`
}

type Store struct {
	collection *mongo.Collection
	workers    chan struct{}
	dummy      string
}

// NewStore 为未知账号准备等成本的验证材料，不写入默认账号。
func NewStore(db *mongo.Database) (*Store, error) {
	// 16 字节随机值编码成 32 位十六进制口令，符合统一输入上限。
	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	dummy, err := Hash(hex.EncodeToString(secret))
	if err != nil {
		return nil, err
	}
	return &Store{collection: db.Collection("accounts"), workers: make(chan struct{}, 2), dummy: dummy}, nil
}

func (s *Store) acquire(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case s.workers <- struct{}{}:
		return func() { <-s.workers }, nil
	default:
		return nil, ErrBusy
	}
}

// Register 通过唯一 _id 原子创建新身份；永不读取或认领旧 players 文档。
func (s *Store) Register(ctx context.Context, name, password string) (Account, error) {
	key, display, err := Normalize(name)
	if err != nil {
		return Account{}, err
	}
	if err = ValidatePassword(password); err != nil {
		return Account{}, err
	}
	release, err := s.acquire(ctx)
	if err != nil {
		return Account{}, err
	}
	defer release()
	if err = s.collection.FindOne(ctx, bson.M{"_id": key}).Err(); err == nil {
		return Account{}, ErrAccountExists
	} else if !errors.Is(err, mongo.ErrNoDocuments) {
		return Account{}, err
	}
	hash, err := Hash(password)
	if err != nil {
		return Account{}, err
	}
	id := make([]byte, 16)
	if _, err = rand.Read(id); err != nil {
		return Account{}, err
	}
	account := Account{Key: key, UserID: hex.EncodeToString(id), Name: display, PasswordHash: hash, CreatedAt: time.Now().UTC()}
	if _, err = s.collection.InsertOne(ctx, account); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return Account{}, ErrAccountExists
		}
		return Account{}, err
	}
	return account, nil
}

// Login 只有口令验证通过才返回公共身份，未知账号和错密码使用相同错误。
func (s *Store) Login(ctx context.Context, name, password string) (Account, error) {
	key, _, err := Normalize(name)
	if err != nil {
		return Account{}, err
	}
	if err = ValidatePassword(password); err != nil {
		return Account{}, err
	}
	release, err := s.acquire(ctx)
	if err != nil {
		return Account{}, err
	}
	defer release()
	var account Account
	err = s.collection.FindOne(ctx, bson.M{"_id": key}).Decode(&account)
	if errors.Is(err, mongo.ErrNoDocuments) {
		_ = Verify(s.dummy, password)
		return Account{}, ErrInvalidCredentials
	}
	if err != nil {
		return Account{}, err
	}
	if !Verify(account.PasswordHash, password) {
		return Account{}, ErrInvalidCredentials
	}
	if err = ctx.Err(); err != nil {
		return Account{}, err
	}
	return account, nil
}
