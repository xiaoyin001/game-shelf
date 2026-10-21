// 文件说明：实现 db 内部 TCP 服务的玩家持久化协议处理逻辑。
package main

import (
	"context"
	"log/slog"
	"time"

	"game-shelf/internal/config"
	"game-shelf/internal/protocol"
	"game-shelf/internal/transport"
	"game-shelf/pkg/contracts"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type DBService struct {
	app config.App
	log *slog.Logger
	db  *mongo.Database
}

func (s *DBService) Handle(ctx context.Context, _ *transport.Conn, msg protocol.Message) (protocol.Message, bool) {
	switch msg.ProtoID {
	case contracts.LGDB_PingReq:
		return pingReply(contracts.DBLG_PingResp, msg.Seq), true
	case contracts.LGDB_UpsertPlayerReq:
		var req contracts.UpsertPlayerReq
		if err := protocol.UnmarshalBody(msg, &req); err != nil {
			return errorReply(contracts.SYS_ErrorNotify, msg.Seq, "invalid_upsert_player_req"), true
		}
		var reply contracts.UpsertPlayerResp
		if err := s.upsertPlayer(ctx, &req, &reply); err != nil {
			s.log.Error("upsert player failed", "error", err, "user_id", req.UserID)
			return errorReply(contracts.SYS_ErrorNotify, msg.Seq, "upsert_player_failed"), true
		}
		resp, err := protocol.MarshalMessage(contracts.DBLG_UpsertPlayerResp, msg.Seq, &reply)
		return resp, err == nil
	default:
		return errorReply(contracts.SYS_ErrorNotify, msg.Seq, "unknown_proto"), true
	}
}

func (s *DBService) upsertPlayer(ctx context.Context, req *contracts.UpsertPlayerReq, reply *contracts.UpsertPlayerResp) error {
	now := time.Now().UTC()
	_, err := s.db.Collection("players").UpdateOne(ctx,
		bson.M{"user_id": req.UserID},
		bson.M{
			"$set": bson.M{
				"account":    req.Account,
				"channel":    req.Channel,
				"updated_at": now,
			},
			"$setOnInsert": bson.M{
				"user_id":    req.UserID,
				"created_at": now,
			},
		},
		options.UpdateOne().SetUpsert(true),
	)
	if err != nil {
		return err
	}
	reply.UserID = req.UserID
	return nil
}

func pingReply(protoID, seq uint32) protocol.Message {
	resp, _ := protocol.MarshalMessage(protoID, seq, &contracts.PingResp{UnixMillis: time.Now().UnixMilli()})
	return resp
}

func errorReply(protoID, seq uint32, message string) protocol.Message {
	resp, _ := protocol.MarshalMessage(protoID, seq, &contracts.ErrorNotify{Code: 1, Message: message})
	return resp
}
