// 文件说明：提供浏览器 WebSocket 接入、单次 token 认证及 lobby 世界协议转发。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"game-shelf/internal/discovery"
	"game-shelf/internal/session"
	"game-shelf/internal/transport"
	"game-shelf/pkg/contracts"

	"github.com/coder/websocket"
)

type browserCommand struct {
	Version  int     `json:"version"`
	Type     string  `json:"type"`
	Token    string  `json:"token,omitempty"`
	DX       float64 `json:"dx,omitempty"`
	DY       float64 `json:"dy,omitempty"`
	Text     string  `json:"text,omitempty"`
	NPCID    string  `json:"npc_id,omitempty"`
	ChoiceID string  `json:"choice_id,omitempty"`
}
type browserEvent struct {
	Version int                    `json:"version"`
	Type    string                 `json:"type"`
	UserID  string                 `json:"user_id,omitempty"`
	Error   string                 `json:"error,omitempty"`
	State   *contracts.WorldState  `json:"state,omitempty"`
	Quest   *contracts.WorldQuest  `json:"quest,omitempty"`
	Dialog  *contracts.WorldDialog `json:"dialog,omitempty"`
	Book    *contracts.FieldBook   `json:"book,omitempty"`
}
type webSession struct {
	online session.OnlineSession
	lobby  *transport.Client
}

func (s *GateService) serveWebSocket(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/ws" {
		http.NotFound(w, r)
		return
	}
	select {
	case s.wsSlots <- struct{}{}:
		defer func() { <-s.wsSlots }()
	default:
		http.Error(w, "gate_full", http.StatusServiceUnavailable)
		return
	}
	// 始终先做精确来源校验，不能让库的同源默认值绕过显式允许列表。
	origin := r.Header.Get("Origin")
	allowed := false
	for _, host := range s.app.WebSocket.Origins {
		if origin == "http://"+host || origin == "https://"+host {
			allowed = true
			break
		}
	}
	if !allowed {
		http.Error(w, "origin_not_allowed", http.StatusForbidden)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: s.app.WebSocket.Origins})
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(4096)
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(runCtx, func() { _ = conn.CloseNow() })
	defer stop()

	authCtx, authCancel := context.WithTimeout(runCtx, 5*time.Second)
	first, err := readBrowserCommand(authCtx, conn)
	authCancel()
	if err != nil {
		return
	}
	if first.Type != "auth" || first.Token == "" {
		s.writeBrowser(runCtx, conn, browserEvent{Type: "error", Error: "auth_required"})
		return
	}
	active, resp, err := s.authenticateBrowser(runCtx, conn, first.Token)
	if err != nil {
		s.writeBrowser(runCtx, conn, browserEvent{Type: "error", Error: err.Error()})
		return
	}
	defer s.leaveBrowser(active, conn)
	if !s.writeBrowser(runCtx, conn, browserEvent{Type: "ready", UserID: active.online.UserID, State: resp.State, Quest: resp.Quest, Book: resp.Book}) {
		return
	}

	commands := make(chan browserCommand, 1)
	go func() {
		defer cancel()
		for {
			readCtx, done := context.WithTimeout(runCtx, s.app.ClientIdleTimeout())
			cmd, err := readBrowserCommand(readCtx, conn)
			done()
			if err != nil {
				return
			}
			select {
			case commands <- cmd:
			case <-runCtx.Done():
				return
			}
		}
	}()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	window := time.Now()
	count := 0
	for {
		select {
		case <-runCtx.Done():
			return
		case cmd := <-commands:
			if time.Since(window) >= time.Second {
				window = time.Now()
				count = 0
			}
			count++
			if count > 30 {
				s.writeBrowser(runCtx, conn, browserEvent{Type: "error", Error: "rate_limited"})
				return
			}
			if cmd.Type == "ping" {
				continue
			}
			if cmd.Type != "input" && cmd.Type != "chat" && cmd.Type != "gather" && cmd.Type != "jump" && cmd.Type != "interact" && cmd.Type != "choose" && cmd.Type != "explore" {
				s.writeBrowser(runCtx, conn, browserEvent{Type: "error", Error: "unknown_action"})
				continue
			}
			reply, err := s.callBrowserWorld(runCtx, active, &contracts.WorldReq{Action: cmd.Type, DX: cmd.DX, DY: cmd.DY, Text: cmd.Text, NPCID: cmd.NPCID, ChoiceID: cmd.ChoiceID})
			if err != nil {
				return
			}
			if reply.Error != "" {
				if !s.writeBrowser(runCtx, conn, browserEvent{Type: "error", Error: reply.Error}) {
					return
				}
			} else if reply.Dialog != nil {
				if !s.writeBrowser(runCtx, conn, browserEvent{Type: "dialog", State: reply.State, Quest: reply.Quest, Dialog: reply.Dialog, Book: reply.Book}) {
					return
				}
			}

		case <-ticker.C:
			reply, err := s.callBrowserWorld(runCtx, active, &contracts.WorldReq{Action: "poll"})
			if err != nil {
				return
			}
			if reply.Error != "" {
				s.writeBrowser(runCtx, conn, browserEvent{Type: "error", Error: reply.Error})
				return
			}
			if !s.writeBrowser(runCtx, conn, browserEvent{Type: "state", State: reply.State, Quest: reply.Quest, Book: reply.Book}) {
				return
			}
		}
	}
}

func readBrowserCommand(ctx context.Context, conn *websocket.Conn) (browserCommand, error) {
	kind, data, err := conn.Read(ctx)
	if err != nil {
		return browserCommand{}, err
	}
	if kind != websocket.MessageText {
		return browserCommand{}, errors.New("text_required")
	}
	var cmd browserCommand
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cmd); err != nil {
		return cmd, err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return cmd, errors.New("trailing_json")
	}
	if cmd.Version != 1 {
		return cmd, errors.New("unsupported_version")
	}
	return cmd, nil
}

func (s *GateService) authenticateBrowser(ctx context.Context, conn *websocket.Conn, token string) (*webSession, *contracts.WorldResp, error) {
	// 本机 gate 的认证、顶号和进入世界按顺序提交，避免旧认证覆盖新连接。
	s.authMu.Lock()
	defer s.authMu.Unlock()
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	value, err := s.store.ConsumeLoginToken(callCtx, token)
	if err != nil {
		return nil, nil, errors.New("invalid_or_expired_token")
	}
	if value.GateInstanceID != s.app.Service.InstanceID {
		return nil, nil, errors.New("wrong_gate")
	}
	lobbies, err := discovery.ListReady(callCtx, s.etcd, s.app.Discovery.BasePath, "lobby")
	if err != nil || len(lobbies) != 1 {
		return nil, nil, errors.New("single_world_unavailable")
	}
	online, previous, err := s.store.CreateOnlineSession(callCtx, value, s.app.OnlineSessionTTL())
	if err != nil {
		return nil, nil, errors.New("create_session_failed")
	}
	if previous != nil {
		s.kickPrevious(callCtx, previous)
	}
	active := &webSession{online: online, lobby: s.lobbyClient(lobbies[0])}
	reply, err := s.callBrowserWorld(callCtx, active, &contracts.WorldReq{Action: "enter", Name: online.Account})
	if err != nil || reply.Error != "" {
		_ = s.store.EndOnlineSession(callCtx, online.SessionID, online.UserID)
		if err != nil {
			return nil, nil, errors.New("world_unavailable")
		}
		return nil, nil, fmt.Errorf("%s", reply.Error)
	}
	s.mu.Lock()
	s.webClients[online.UserID] = conn
	s.mu.Unlock()
	return active, reply, nil
}

func (s *GateService) callBrowserWorld(ctx context.Context, active *webSession, req *contracts.WorldReq) (*contracts.WorldResp, error) {
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := s.store.RenewOnlineSession(callCtx, active.online.SessionID, s.app.OnlineSessionTTL()); err != nil {
		return nil, err
	}
	req.UserID = active.online.UserID
	req.SessionID = active.online.SessionID
	var reply contracts.WorldResp
	err := active.lobby.Call(callCtx, contracts.GTLB_WorldReq, contracts.LBGT_WorldResp, req, &reply)
	return &reply, err
}

func (s *GateService) leaveBrowser(active *webSession, conn *websocket.Conn) {
	s.mu.Lock()
	if s.webClients[active.online.UserID] == conn {
		delete(s.webClients, active.online.UserID)
	}
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	// leave 仍带旧会话，world 会拒绝它删除已经顶替的新角色。
	req := &contracts.WorldReq{UserID: active.online.UserID, SessionID: active.online.SessionID, Action: "leave"}
	var reply contracts.WorldResp
	_ = active.lobby.Call(ctx, contracts.GTLB_WorldReq, contracts.LBGT_WorldResp, req, &reply)
	_ = s.store.EndOnlineSession(ctx, active.online.SessionID, active.online.UserID)
}

func (s *GateService) writeBrowser(ctx context.Context, conn *websocket.Conn, event browserEvent) bool {
	event.Version = 1
	data, err := json.Marshal(event)
	if err != nil {
		return false
	}
	writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return conn.Write(writeCtx, websocket.MessageText, data) == nil
}
