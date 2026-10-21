// 文件说明：封装 Redis 客户端连接和连通性检查逻辑。
package redisx

import (
	"context"
	"time"

	"game-shelf/internal/config"

	"github.com/redis/go-redis/v9"
)

func Connect(ctx context.Context, cfg config.Redis) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}
