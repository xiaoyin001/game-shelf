// 文件说明：维护单世界的在线角色、会话替换和退出租约，不提前模拟玩法。
package world

import (
	"game-shelf/pkg/contracts"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	width     = 1200.0
	height    = 800.0
	playerTTL = 5 * time.Second
)

type player struct {
	state    contracts.WorldPlayer
	session  string
	lastSeen time.Time
}

// World 统一持有在线角色，快照与修改在同一锁中完成。
type World struct {
	mu      sync.Mutex
	players map[string]*player
}

// New 创建没有在线玩家的世界。
func New() *World { return &World{players: make(map[string]*player)} }

// Apply 使用服务器时间校验会话并复制当前快照。
func (w *World) Apply(req *contracts.WorldReq, now time.Time) *contracts.WorldResp {
	w.mu.Lock()
	defer w.mu.Unlock()
	for id, p := range w.players {
		if now.Sub(p.lastSeen) > playerTTL {
			delete(w.players, id)
		}
	}
	if req.UserID == "" || req.SessionID == "" {
		return failure("unauthenticated")
	}
	if req.Action == "enter" {
		name := strings.TrimSpace(req.Name)
		if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 24 {
			return failure("invalid_name")
		}
		if p := w.players[req.UserID]; p != nil && p.session == req.SessionID {
			p.lastSeen = now
			return &contracts.WorldResp{State: w.snapshot(now)}
		}
		if w.players[req.UserID] == nil && len(w.players) >= 100 {
			return failure("world_full")
		}
		w.players[req.UserID] = &player{state: contracts.WorldPlayer{ID: req.UserID, Name: name, X: 560, Y: 400}, session: req.SessionID, lastSeen: now}
		return &contracts.WorldResp{State: w.snapshot(now)}
	}
	p := w.players[req.UserID]
	if p == nil || p.session != req.SessionID {
		return failure("session_replaced_or_expired")
	}
	p.lastSeen = now
	switch req.Action {
	case "poll":
	case "leave":
		delete(w.players, req.UserID)
	default:
		return failure("unknown_action")
	}
	return &contracts.WorldResp{State: w.snapshot(now)}
}
func (w *World) snapshot(now time.Time) *contracts.WorldState {
	state := &contracts.WorldState{TimeMS: now.UnixMilli(), Width: width, Height: height, Players: make([]*contracts.WorldPlayer, 0, len(w.players))}
	for _, p := range w.players {
		copy := p.state
		state.Players = append(state.Players, &copy)
	}
	sort.Slice(state.Players, func(i, j int) bool { return state.Players[i].ID < state.Players[j].ID })
	return state
}
func failure(message string) *contracts.WorldResp { return &contracts.WorldResp{Error: message} }
