// 文件说明：维护权威物理与隔离的限时挑战，旧探索档不再参与世界进入。
package world

import (
	"context"
	"game-shelf/internal/challenge"
	"game-shelf/pkg/contracts"
	"game-shelf/pkg/scene"
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
	vz       float64
	round    *challenge.Round
}

func (p *player) scope() string {
	if p.round != nil {
		return p.round.ID()
	}
	return ""
}

type chatEntry struct {
	value contracts.WorldChat
	scope string
}

// World 的世界锁同时串行化物理与局内物品，网络 I/O 不在此锁内执行。
type World struct {
	mu          sync.Mutex
	players     map[string]*player
	chats       []chatEntry
	chatID      int64
	nodes       []contracts.WorldNode
	layout      scene.Layout
	lastPhysics time.Time
}

// New 不读取或覆盖旧 field_books，保留账号和原有 Mongo 数据。
func New() *World {
	w := &World{players: make(map[string]*player), layout: scene.Default()}
	for _, n := range w.layout.Nodes {
		w.nodes = append(w.nodes, contracts.WorldNode{ID: n.ID, X: n.X, Y: n.Y})
	}
	return w
}

func (w *World) Apply(req *contracts.WorldReq, now time.Time) *contracts.WorldResp {
	return w.ApplyContext(context.Background(), req, now)
}

// ApplyContext 使用 gate 提供的会话身份，客户端不能传入分数、时钟或坐标。
func (w *World) ApplyContext(ctx context.Context, req *contracts.WorldReq, now time.Time) *contracts.WorldResp {
	if ctx.Err() != nil {
		return failure("world_unavailable")
	}
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
	w.advance(now)
	for _, p := range w.players {
		if p.round != nil {
			p.round.Advance(now)
		}
	}
	if req.Action == "enter" {
		name := strings.TrimSpace(req.Name)
		if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 24 {
			return failure("invalid_name")
		}
		if p := w.players[req.UserID]; p != nil {
			if p.session != req.SessionID {
				p.session = req.SessionID
			}
			p.lastSeen = now
			return w.response(p, now)
		}
		if len(w.players) >= 100 {
			return failure("world_full")
		}
		x, y, ok := w.spawn(req.UserID)
		if !ok {
			return failure("world_full")
		}
		p := &player{state: contracts.WorldPlayer{ID: req.UserID, Name: name, X: x, Y: y, Grounded: true}, session: req.SessionID, lastSeen: now, lastMove: now}
		w.players[req.UserID] = p
		return w.response(p, now)
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
	case "jump":
		if !p.state.Grounded || !w.supported(p) {
			return failure("jump_not_grounded")
		}
		p.vz = jumpSpeed
		p.state.Grounded = false
	case "start_solo":
		if p.round != nil && p.round.Phase() == "running" {
			return failure("already_in_round")
		}
		p.round = challenge.New(p.state.ID, p.state.Name, now, w.layout.Workshop)
		p.state.X = w.layout.Workshop.SpawnX
		p.state.Y = w.layout.Workshop.SpawnY
		p.state.Z = 0
		p.state.Grounded = true
		p.vz = 0
		p.lastMove = now
	case "pick", "put", "return":
		if p.round == nil {
			return failure("no_round")
		}
		pose := p.state
		pose.Grounded = pose.Grounded && w.supported(p)
		if err := p.round.Apply(req.Action, req.RoundID, req.ItemID, pose, now); err != "" {
			return failure(err)
		}
	case "leave_round":
		if p.round != nil {
			x, y, ok := w.spawn(p.state.ID)
			if !ok {
				return failure("world_full")
			}
			p.round.Abandon()
			p.round = nil
			p.state.X, p.state.Y, p.state.Z = x, y, 0
			p.state.Grounded, p.vz, p.lastMove = true, 0, now
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
	return w.response(p, now)
}

// spawn 检查静态实体与同作用域身体；局内玩家不占用大厅出生格。
func (w *World) spawn(userID string) (float64, float64, bool) {
	offset := func(i int) float64 {
		if i%2 == 1 {
			return float64((i+1)/2) * 40
		}
		return -float64(i/2) * 40
	}
	for row := 0; row < 19; row++ {
		for col := 0; col < 29; col++ {
			x, y := 560+offset(col), 400+offset(row)
			if x < 20 || x > width-20 || y < 20 || y > height-20 {
				continue
			}
			probe := &player{state: contracts.WorldPlayer{ID: userID, X: x, Y: y}}
			if !w.blocked(probe, x, y) {
				return x, y, true
			}
		}
	}
	return 0, 0, false
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
	w.chats = append(w.chats, chatEntry{value: contracts.WorldChat{ID: w.chatID, Name: p.state.Name, Text: value}, scope: p.scope()})
	if len(w.chats) > 300 {
		w.chats = w.chats[len(w.chats)-300:]
	}
	return ""
}

func (w *World) response(p *player, now time.Time) *contracts.WorldResp {
	state := &contracts.WorldState{SceneVersion: w.layout.Version, TimeMS: now.UnixMilli(), Width: width, Height: height,
		Players: make([]*contracts.WorldPlayer, 0), Chats: make([]*contracts.WorldChat, 0), Nodes: make([]*contracts.WorldNode, 0, len(w.nodes))}
	for _, other := range w.players {
		if other.scope() == p.scope() {
			v := other.state
			state.Players = append(state.Players, &v)
		}
	}
	for _, n := range w.nodes {
		v := n
		state.Nodes = append(state.Nodes, &v)
	}
	for _, c := range w.chats {
		if c.scope == p.scope() {
			v := c.value
			state.Chats = append(state.Chats, &v)
		}
	}
	if len(state.Chats) > 30 {
		state.Chats = state.Chats[len(state.Chats)-30:]
	}
	sort.Slice(state.Players, func(i, j int) bool { return state.Players[i].ID < state.Players[j].ID })
	resp := &contracts.WorldResp{State: state}
	if p.round != nil {
		resp.Challenge = p.round.Snapshot()
	}
	return resp
}
func failure(message string) *contracts.WorldResp { return &contracts.WorldResp{Error: message} }
