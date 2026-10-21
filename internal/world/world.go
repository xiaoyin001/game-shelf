// 文件说明：维护单世界在线状态与服务器时间约束的权威移动。
package world

import (
	"game-shelf/pkg/contracts"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	width     = 1200.0
	height    = 800.0
	speed     = 180.0
	playerTTL = 5 * time.Second
)

type player struct {
	state    contracts.WorldPlayer
	session  string
	lastSeen time.Time
	lastMove time.Time
	lastChat time.Time
}

// World 统一持有在线角色，快照与修改在同一锁中完成。
type World struct {
	mu      sync.Mutex
	players map[string]*player
	chats   []contracts.WorldChat
	chatID  int64
	nodes   []contracts.WorldNode
}

// New 创建没有在线玩家的世界。
func New() *World {
	return &World{players: make(map[string]*player), nodes: []contracts.WorldNode{
		{ID: "crystal-1", X: 620, Y: 400}, {ID: "crystal-2", X: 300, Y: 260}, {ID: "crystal-3", X: 900, Y: 260}, {ID: "crystal-4", X: 350, Y: 600}, {ID: "crystal-5", X: 850, Y: 600},
	}}
}

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
		w.players[req.UserID] = &player{state: contracts.WorldPlayer{ID: req.UserID, Name: name, X: 560, Y: 400}, session: req.SessionID, lastSeen: now, lastMove: now}
		return &contracts.WorldResp{State: w.snapshot(now)}
	}
	p := w.players[req.UserID]
	if p == nil || p.session != req.SessionID {
		return failure("session_replaced_or_expired")
	}
	p.lastSeen = now
	switch req.Action {
	case "poll":
	case "input":
		if err := w.move(p, req.DX, req.DY, now); err != "" {
			return failure(err)
		}
	case "gather":
		if err := w.gather(p, now); err != "" {
			return failure(err)
		}
	case "chat":
		if err := w.chat(p, req.Text, now); err != "" {
			return failure(err)
		}
	case "leave":
		delete(w.players, req.UserID)
	default:
		return failure("unknown_action")
	}
	return &contracts.WorldResp{State: w.snapshot(now)}
}
func (w *World) move(p *player, dx, dy float64, now time.Time) string {
	if math.IsNaN(dx) || math.IsNaN(dy) || math.IsInf(dx, 0) || math.IsInf(dy, 0) {
		return "invalid_direction"
	}
	length := math.Hypot(dx, dy)
	if math.IsInf(length, 0) {
		return "invalid_direction"
	}
	if length > 1 {
		dx /= length
		dy /= length
	}
	// 每次最多承认 150ms 的位移，空闲与网络堵塞不能积攒移动额度。
	dt := math.Max(0, math.Min(now.Sub(p.lastMove).Seconds(), 0.15))
	p.lastMove = now
	p.state.X = math.Max(20, math.Min(width-20, p.state.X+dx*speed*dt))
	p.state.Y = math.Max(20, math.Min(height-20, p.state.Y+dy*speed*dt))
	return ""
}

func (w *World) gather(p *player, now time.Time) string {
	for i := range w.nodes {
		n := &w.nodes[i]
		if math.Hypot(p.state.X-n.X, p.state.Y-n.Y) <= 72 && now.UnixMilli() >= n.ReadyAtMS {
			n.ReadyAtMS = now.Add(10 * time.Second).UnixMilli()
			p.state.Score++
			return ""
		}
	}
	return "no_resource_in_range"
}

func (w *World) chat(p *player, raw string, now time.Time) string {
	value := strings.TrimSpace(raw)
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 140 {
		return "invalid_chat"
	}
	if !p.lastChat.IsZero() && now.Sub(p.lastChat) < 800*time.Millisecond {
		return "chat_too_fast"
	}
	p.lastChat = now
	w.chatID++
	w.chats = append(w.chats, contracts.WorldChat{ID: w.chatID, Name: p.state.Name, Text: value})
	if len(w.chats) > 30 {
		w.chats = w.chats[len(w.chats)-30:]
	}
	return ""
}

func (w *World) snapshot(now time.Time) *contracts.WorldState {
	state := &contracts.WorldState{TimeMS: now.UnixMilli(), Width: width, Height: height, Players: make([]*contracts.WorldPlayer, 0, len(w.players)), Chats: make([]*contracts.WorldChat, 0, len(w.chats)), Nodes: make([]*contracts.WorldNode, 0, len(w.nodes))}
	for _, p := range w.players {
		copy := p.state
		state.Players = append(state.Players, &copy)
	}
	for _, n := range w.nodes {
		copy := n
		state.Nodes = append(state.Nodes, &copy)
	}
	for _, c := range w.chats {
		copy := c
		state.Chats = append(state.Chats, &copy)
	}
	sort.Slice(state.Players, func(i, j int) bool { return state.Players[i].ID < state.Players[j].ID })
	return state
}
func failure(message string) *contracts.WorldResp { return &contracts.WorldResp{Error: message} }
