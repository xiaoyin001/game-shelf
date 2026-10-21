// 文件说明：封装 login token、在线 session 和 Redis key 生命周期管理。
package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "gs:"

type LoginToken struct {
	Token          string    `json:"token"`
	UserID         string    `json:"user_id"`
	Account        string    `json:"account"`
	Channel        string    `json:"channel"`
	GateInstanceID string    `json:"gate_instance_id"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
}

type OnlineSession struct {
	SessionID      string    `json:"session_id"`
	UserID         string    `json:"user_id"`
	Account        string    `json:"account"`
	Channel        string    `json:"channel"`
	GateInstanceID string    `json:"gate_instance_id"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
}

type Store struct {
	redis *redis.Client
}

func NewStore(redis *redis.Client) *Store {
	return &Store{redis: redis}
}

func (s *Store) CreateLoginToken(ctx context.Context, userID, account, channel, gateInstanceID string, ttl time.Duration) (LoginToken, error) {
	token, err := randomHex(32)
	if err != nil {
		return LoginToken{}, err
	}
	now := time.Now().UTC()
	value := LoginToken{
		Token:          token,
		UserID:         userID,
		Account:        account,
		Channel:        channel,
		GateInstanceID: gateInstanceID,
		CreatedAt:      now,
		ExpiresAt:      now.Add(ttl),
	}
	data, err := json.Marshal(value)
	if err != nil {
		return LoginToken{}, err
	}
	return value, s.redis.Set(ctx, loginTokenKey(token), data, ttl).Err()
}

func (s *Store) ConsumeLoginToken(ctx context.Context, token string) (LoginToken, error) {
	key := loginTokenKey(token)
	data, err := s.redis.GetDel(ctx, key).Bytes()
	if err != nil {
		return LoginToken{}, err
	}
	var value LoginToken
	if err := json.Unmarshal(data, &value); err != nil {
		return LoginToken{}, err
	}
	return value, nil
}

func (s *Store) CreateOnlineSession(ctx context.Context, token LoginToken, ttl time.Duration) (OnlineSession, *OnlineSession, error) {
	sessionID, err := randomHex(24)
	if err != nil {
		return OnlineSession{}, nil, err
	}
	now := time.Now().UTC()
	next := OnlineSession{
		SessionID:      sessionID,
		UserID:         token.UserID,
		Account:        token.Account,
		Channel:        token.Channel,
		GateInstanceID: token.GateInstanceID,
		CreatedAt:      now,
		ExpiresAt:      now.Add(ttl),
	}

	data, err := json.Marshal(next)
	if err != nil {
		return OnlineSession{}, nil, err
	}
	// 获取旧会话和发布新会话必须原子完成，避免并发登录丢失顶号关系。
	old, err := s.redis.Eval(ctx, `
 local previous = redis.call('GET', KEYS[2])
 local value = ''
 if previous then value = redis.call('GET', ARGV[4] .. previous) or '' end
 redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[3])
 redis.call('SET', KEYS[2], ARGV[2], 'PX', ARGV[3])
 return value
 `, []string{onlineSessionKey(sessionID), userSessionKey(token.UserID)},
		data, sessionID, ttl.Milliseconds(), keyPrefix+"online_session:").Text()
	if err != nil {
		return OnlineSession{}, nil, err
	}
	var previous *OnlineSession
	if old != "" {
		var p OnlineSession
		if json.Unmarshal([]byte(old), &p) == nil {
			previous = &p
		}
	}
	return next, previous, nil
}

func (s *Store) GetOnlineSession(ctx context.Context, sessionID string) (OnlineSession, error) {
	data, err := s.redis.Get(ctx, onlineSessionKey(sessionID)).Bytes()
	if err != nil {
		return OnlineSession{}, err
	}
	var value OnlineSession
	if err := json.Unmarshal(data, &value); err != nil {
		return OnlineSession{}, err
	}
	return value, nil
}

func (s *Store) RenewOnlineSession(ctx context.Context, sessionID string, ttl time.Duration) error {
	session, err := s.GetOnlineSession(ctx, sessionID)
	if err != nil {
		return err
	}
	session.ExpiresAt = time.Now().UTC().Add(ttl)
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	ok, err := s.redis.Eval(ctx, `
 if redis.call('GET', KEYS[2]) ~= ARGV[2] or redis.call('EXISTS', KEYS[1]) == 0 then return 0 end
 redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[3])
 redis.call('PEXPIRE', KEYS[2], ARGV[3])
 return 1
 `, []string{onlineSessionKey(sessionID), userSessionKey(session.UserID)}, data, sessionID, ttl.Milliseconds()).Int()
	if err != nil {
		return err
	}
	if ok != 1 {
		return fmt.Errorf("session_replaced_or_expired")
	}
	return nil
}

// EndOnlineSession 只删除自己的会话，旧连接退出不能删除新连接的映射。
func (s *Store) EndOnlineSession(ctx context.Context, sessionID, userID string) error {
	return s.redis.Eval(ctx, `
 redis.call('DEL', KEYS[1])
 if redis.call('GET', KEYS[2]) == ARGV[1] then redis.call('DEL', KEYS[2]) end
 return 1
 `, []string{onlineSessionKey(sessionID), userSessionKey(userID)}, sessionID).Err()
}

func loginTokenKey(token string) string {
	return keyPrefix + "login_token:" + token
}

func onlineSessionKey(sessionID string) string {
	return keyPrefix + "online_session:" + sessionID
}

func userSessionKey(userID string) string {
	return keyPrefix + "user_session:" + userID
}

func randomHex(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
