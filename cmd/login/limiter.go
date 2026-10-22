// 文件说明：用 Redis 原子窗口限制口令入口，键不记录明文账号或密码且自动过期。
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net"
)

func (h *LoginHTTP) allowAuth(ctx context.Context, remote, key string) (bool, error) {
	ip, _, err := net.SplitHostPort(remote)
	if err != nil {
		ip = remote
	}
	digest := func(s string) string { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
	value, err := h.redis.Eval(ctx, `
 for i=1,2 do
  local n=redis.call('INCR',KEYS[i])
  if n==1 then redis.call('EXPIRE',KEYS[i],60) end
 end
 if tonumber(redis.call('GET',KEYS[1]))>120 or tonumber(redis.call('GET',KEYS[2]))>12 then return 0 end
 return 1
 `, []string{"gs:auth:ip:" + digest(ip), "gs:auth:account:" + digest(key)}).Int()
	return value == 1, err
}
