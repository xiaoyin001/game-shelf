// 文件说明：封装 MongoDB 客户端连接和连通性检查逻辑。
package mongox

import (
	"context"
	"time"

	"game-shelf/internal/config"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

func Connect(ctx context.Context, cfg config.Mongo) (*mongo.Client, error) {
	client, err := mongo.Connect(options.Client().
		ApplyURI(cfg.URI).
		SetServerSelectionTimeout(2 * time.Second))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}
	return client, nil
}
