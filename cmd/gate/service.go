// 文件说明：实现 gate 的客户端 TCP 协议、内部控制协议和业务转发逻辑。
package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"game-shelf/internal/config"
	"game-shelf/internal/discovery"
	"game-shelf/internal/protocol"
	"game-shelf/internal/session"
	"game-shelf/internal/transport"
	"game-shelf/pkg/contracts"

	"github.com/coder/websocket"
	"github.com/redis/go-redis/v9"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type GateService struct {
	app   config.App
	log   *slog.Logger
	etcd  *clientv3.Client
	store *session.Store

	authMu     sync.Mutex
	wsSlots    chan struct{}
	webClients map[string]*websocket.Conn
	mu         sync.Mutex
	clients    map[string]*transport.Conn

	internalMu   sync.Mutex
	lobbyClients map[string]*transport.Client
	gateClients  map[string]*transport.Client
}

func NewGateService(app config.App, log *slog.Logger, etcd *clientv3.Client, redis *redis.Client) *GateService {
	return &GateService{
		app:          app,
		log:          log,
		etcd:         etcd,
		store:        session.NewStore(redis),
		clients:      map[string]*transport.Conn{},
		webClients:   map[string]*websocket.Conn{},
		wsSlots:      make(chan struct{}, app.Limit.MaxConn),
		lobbyClients: map[string]*transport.Client{},
		gateClients:  map[string]*transport.Client{},
	}
}

func (s *GateService) HandleClient(ctx context.Context, conn *transport.Conn, msg protocol.Message) (protocol.Message, bool) {
	switch msg.ProtoID {
	case contracts.CS_PingReq:
		return pingReply(contracts.SC_PingResp, msg.Seq), true
	case contracts.CS_GateAuthReq:
		return s.authClient(ctx, conn, msg), true
	case contracts.CS_LobbyListReq:
		return s.lobbyList(ctx, msg), true
	default:
		return errorReply(contracts.SC_ErrorNotify, msg.Seq, "unknown_proto"), true
	}
}

func (s *GateService) HandleInternal(ctx context.Context, conn *transport.Conn, msg protocol.Message) (protocol.Message, bool) {
	switch msg.ProtoID {
	case contracts.GTGT_PingReq:
		return pingReply(contracts.GTGT_PingResp, msg.Seq), true
	case contracts.GTGT_KickUserReq:
		var req contracts.KickUserReq
		if err := protocol.UnmarshalBody(msg, &req); err != nil {
			return errorReply(contracts.GTGT_KickUserResp, msg.Seq, "invalid_kick_user_req"), true
		}
		s.kickLocal(req.UserID, req.Reason)
		resp, _ := protocol.MarshalMessage(contracts.GTGT_KickUserResp, msg.Seq, &contracts.KickUserResp{Code: 0, Message: "ok"})
		return resp, true
	default:
		return errorReply(contracts.SYS_ErrorNotify, msg.Seq, "unknown_proto"), true
	}
}

func (s *GateService) authClient(ctx context.Context, conn *transport.Conn, msg protocol.Message) protocol.Message {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	var req contracts.GateAuthReq
	if err := protocol.UnmarshalBody(msg, &req); err != nil {
		return gateAuthResp(msg.Seq, 1, "invalid_auth_req", "", "")
	}
	token, err := s.store.ConsumeLoginToken(ctx, req.Token)
	if err != nil {
		return gateAuthResp(msg.Seq, 1, "invalid_or_expired_token", "", "")
	}
	if token.GateInstanceID != "" && token.GateInstanceID != s.app.Service.InstanceID {
		return gateAuthResp(msg.Seq, 1, "wrong_gate", "", "")
	}
	online, previous, err := s.store.CreateOnlineSession(ctx, token, s.app.OnlineSessionTTL())
	if err != nil {
		s.log.Error("create online session failed", "error", err, "user_id", token.UserID)
		return gateAuthResp(msg.Seq, 1, "create_session_failed", "", "")
	}
	if previous != nil && !s.app.Session.MultiLogin {
		s.kickPrevious(ctx, previous)
	}
	s.bindClient(online.UserID, conn)
	return gateAuthResp(msg.Seq, 0, "ok", online.UserID, online.SessionID)
}

func (s *GateService) lobbyList(ctx context.Context, msg protocol.Message) protocol.Message {
	var req contracts.LobbyListReq
	if err := protocol.UnmarshalBody(msg, &req); err != nil {
		return errorReply(contracts.SC_ErrorNotify, msg.Seq, "invalid_lobby_list_req")
	}
	lobbies, err := discovery.ListReady(ctx, s.etcd, s.app.Discovery.BasePath, "lobby")
	if err != nil || len(lobbies) == 0 {
		return errorReply(contracts.SC_ErrorNotify, msg.Seq, "lobby_unavailable")
	}
	client := s.lobbyClient(lobbies[0])

	var reply contracts.LobbyListResp
	if err := client.Call(ctx, contracts.GTLB_LobbyListReq, contracts.LBGT_LobbyListResp, &req, &reply); err != nil {
		s.log.Error("lobby call failed", "error", err)
		return errorReply(contracts.SC_ErrorNotify, msg.Seq, "lobby_unavailable")
	}
	resp, _ := protocol.MarshalMessage(contracts.SC_LobbyListResp, msg.Seq, &reply)
	return resp
}

func (s *GateService) bindClient(userID string, conn *transport.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old := s.clients[userID]; old != nil {
		_ = old.Write(kickNotify("new_login"))
		_ = old.Close()
	}
	s.clients[userID] = conn
}

func (s *GateService) kickLocal(userID, reason string) {
	s.mu.Lock()
	conn := s.clients[userID]
	web := s.webClients[userID]
	delete(s.clients, userID)
	delete(s.webClients, userID)
	s.mu.Unlock()
	if conn != nil {
		_ = conn.Write(kickNotify(reason))
		_ = conn.Close()
	}
	if web != nil {
		_ = web.CloseNow()
	}
}

func (s *GateService) kickPrevious(ctx context.Context, previous *session.OnlineSession) {
	if previous.GateInstanceID == s.app.Service.InstanceID {
		s.kickLocal(previous.UserID, "new_login")
		return
	}
	gates, err := discovery.ListReady(ctx, s.etcd, s.app.Discovery.BasePath, "gate")
	if err != nil {
		return
	}
	for _, gate := range gates {
		if gate.InstanceID != previous.GateInstanceID {
			continue
		}
		client := s.gateClient(gate)
		var reply contracts.KickUserResp
		_ = client.Call(ctx, contracts.GTGT_KickUserReq, contracts.GTGT_KickUserResp, &contracts.KickUserReq{
			UserID:    previous.UserID,
			SessionID: previous.SessionID,
			Reason:    "new_login",
		}, &reply)
		return
	}
}

func (s *GateService) lobbyClient(instance discovery.Instance) *transport.Client {
	s.internalMu.Lock()
	defer s.internalMu.Unlock()
	if client := s.lobbyClients[instance.InstanceID]; client != nil {
		return client
	}
	client := transport.NewClient(instance.InternalAddr, 3*time.Second, s.app.Limit.MaxBodyBytes, s.log)
	client.StartHeartbeat(context.Background(), contracts.GTLB_PingReq, contracts.LBGT_PingResp, s.app.InternalHeartbeatInterval())
	s.lobbyClients[instance.InstanceID] = client
	return client
}

func (s *GateService) gateClient(instance discovery.Instance) *transport.Client {
	s.internalMu.Lock()
	defer s.internalMu.Unlock()
	if client := s.gateClients[instance.InstanceID]; client != nil {
		return client
	}
	client := transport.NewClient(instance.InternalAddr, 3*time.Second, s.app.Limit.MaxBodyBytes, s.log)
	client.StartHeartbeat(context.Background(), contracts.GTGT_PingReq, contracts.GTGT_PingResp, s.app.InternalHeartbeatInterval())
	s.gateClients[instance.InstanceID] = client
	return client
}

func gateAuthResp(seq uint32, code int32, message, userID, sessionID string) protocol.Message {
	resp, _ := protocol.MarshalMessage(contracts.SC_GateAuthResp, seq, &contracts.GateAuthResp{
		Code: code, Message: message, UserID: userID, SessionID: sessionID,
	})
	return resp
}

func pingReply(protoID, seq uint32) protocol.Message {
	resp, _ := protocol.MarshalMessage(protoID, seq, &contracts.PingResp{UnixMillis: time.Now().UnixMilli()})
	return resp
}

func errorReply(protoID, seq uint32, message string) protocol.Message {
	resp, _ := protocol.MarshalMessage(protoID, seq, &contracts.ErrorNotify{Code: 1, Message: message})
	return resp
}

func kickNotify(reason string) protocol.Message {
	resp, _ := protocol.MarshalMessage(contracts.SC_KickNotify, 0, &contracts.KickNotify{Reason: reason})
	return resp
}
