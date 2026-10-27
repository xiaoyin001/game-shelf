// 文件说明：实现 lobby 内部 TCP 服务的大厅协议处理逻辑。
package main

import (
	"context"
	clientv3 "go.etcd.io/etcd/client/v3"
	"log/slog"
	"time"

	"game-shelf/internal/config"
	"game-shelf/internal/protocol"
	"game-shelf/internal/transport"
	"game-shelf/internal/world"
	"game-shelf/pkg/contracts"

	"github.com/redis/go-redis/v9"
)

type LobbyService struct {
	app   config.App
	log   *slog.Logger
	redis *redis.Client
	world *world.World
	etcd  *clientv3.Client
	ctx   context.Context
}

func (s *LobbyService) Handle(ctx context.Context, _ *transport.Conn, msg protocol.Message) (protocol.Message, bool) {
	switch msg.ProtoID {
	case contracts.GTLB_PingReq:
		return pingReply(contracts.LBGT_PingResp, msg.Seq), true
	case contracts.GTLB_WorldReq:
		var req contracts.WorldReq
		if err := protocol.UnmarshalBody(msg, &req); err != nil {
			return errorReply(msg.Seq, "invalid_world_req"), true
		}
		reply, err := protocol.MarshalMessage(contracts.LBGT_WorldResp, msg.Seq, s.world.ApplyContext(ctx, &req, time.Now()))
		return reply, err == nil
	case contracts.GTLB_LobbyListReq:
		var req contracts.LobbyListReq
		if err := protocol.UnmarshalBody(msg, &req); err != nil {
			return errorReply(msg.Seq, "invalid_lobby_list_req"), true
		}
		var reply contracts.LobbyListResp
		if err := s.lobbyList(ctx, &req, &reply); err != nil {
			s.log.Error("lobby list failed", "error", err, "user_id", req.UserID)
			return errorReply(msg.Seq, "lobby_list_failed"), true
		}
		resp, err := protocol.MarshalMessage(contracts.LBGT_LobbyListResp, msg.Seq, &reply)
		return resp, err == nil
	default:
		return errorReply(msg.Seq, "unknown_proto"), true
	}
}

func (s *LobbyService) lobbyList(ctx context.Context, req *contracts.LobbyListReq, reply *contracts.LobbyListResp) error {
	// 当前只提供开发大厅列表，不用永久 Redis 集合冒充在线人数。
	reply.UserID = req.UserID
	reply.Rooms = []string{"room-1001", "room-1002"}
	return nil
}

func pingReply(protoID, seq uint32) protocol.Message {
	resp, _ := protocol.MarshalMessage(protoID, seq, &contracts.PingResp{UnixMillis: time.Now().UnixMilli()})
	return resp
}

func errorReply(seq uint32, message string) protocol.Message {
	resp, _ := protocol.MarshalMessage(contracts.SYS_ErrorNotify, seq, &contracts.ErrorNotify{Code: 1, Message: message})
	return resp
}
