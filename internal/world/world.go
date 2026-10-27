// 文件说明：维护单世界在线状态与服务器时间约束的权威移动。
package world

import (
	"context"
	"game-shelf/pkg/contracts"
	"game-shelf/pkg/fieldbook"
	"game-shelf/pkg/scene"
	"hash/fnv"
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
	quest    contracts.WorldQuest
	journal  *contracts.Journal
	vz       float64
}

// World 统一持有在线角色，快照与修改在同一锁中完成。
type World struct {
	mu          sync.Mutex
	players     map[string]*player
	chats       []contracts.WorldChat
	chatID      int64
	nodes       []contracts.WorldNode
	layout      scene.Layout
	lastPhysics time.Time
	books       ProgressStore
	userOps     [64]chan struct{}
}

// New 创建没有在线玩家的世界。
func New() *World { return NewPersistent(nil) }

// NewPersistent 只由生产lobby注入真正存储；无存储的世界不能确认持久探索。
func NewPersistent(books ProgressStore) *World {
	w := &World{players: make(map[string]*player), layout: scene.Default(), books: books}
	for i := range w.userOps {
		w.userOps[i] = make(chan struct{}, 1)
	}
	for _, n := range w.layout.Nodes {
		w.nodes = append(w.nodes, contracts.WorldNode{ID: n.ID, X: n.X, Y: n.Y})
	}
	return w
}

// Apply 使用服务器时间校验会话并复制当前快照。
func (w *World) Apply(req *contracts.WorldReq, now time.Time) *contracts.WorldResp {
	return w.ApplyContext(context.Background(), req, now)
}

// ApplyContext 串行化同账号状态，I/O限时且不持有全局世界锁。
func (w *World) ApplyContext(ctx context.Context, req *contracts.WorldReq, now time.Time) *contracts.WorldResp {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	h := fnv.New32a()
	_, _ = h.Write([]byte(req.UserID))
	slot := w.userOps[h.Sum32()%uint32(len(w.userOps))]
	select {
	case slot <- struct{}{}:
		defer func() { <-slot }()
	case <-ctx.Done():
		return failure("progress_unavailable")
	}
	initial := &contracts.Journal{}
	if req.Action == "enter" && w.books != nil && req.UserID != "" && req.SessionID != "" {
		var err error
		initial, err = w.books.Load(ctx, req.UserID)
		if err != nil || fieldbook.Validate(initial) != nil {
			return failure("progress_unavailable")
		}
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
	if req.Action == "enter" {
		name := strings.TrimSpace(req.Name)
		if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 24 {
			return failure("invalid_name")
		}
		if p := w.players[req.UserID]; p != nil && p.session == req.SessionID {
			p.lastSeen = now
			return w.response(p, now, nil)
		}
		if w.players[req.UserID] == nil && len(w.players) >= 100 {
			return failure("world_full")
		}
		x, y, ok := w.spawn(req.UserID)
		if !ok {
			return failure("world_full")
		}
		w.players[req.UserID] = &player{state: contracts.WorldPlayer{ID: req.UserID, Name: name, X: x, Y: y, Grounded: true}, session: req.SessionID, lastSeen: now, lastMove: now, journal: fieldbook.Clone(initial)}
		return w.response(w.players[req.UserID], now, nil)
	}
	p := w.players[req.UserID]
	if p == nil || p.session != req.SessionID {
		return failure("session_replaced_or_expired")
	}
	p.lastSeen = now
	var dialog *contracts.WorldDialog
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
	case "gather":
		if err := w.gather(p, now); err != "" {
			return failure(err)
		}
	case "explore":
		if err := w.explore(ctx, p); err != "" {
			return failure(err)
		}
	case "interact", "choose":
		var err string
		dialog, err = w.interact(p, req)
		if err == "" && req.Action == "choose" && req.ChoiceID == "trail" {
			if err = w.acceptTrail(ctx, p); err == "" {
				dialog, _ = w.interact(p, req)
			}
		}
		if err != "" {
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
	return w.response(p, now, dialog)
}

// spawn 同时检查静态实体与在线角色，不把新角色塞进障碍或其他身体。
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

func (w *World) gather(p *player, now time.Time) string {
	for i := range w.nodes {
		n := &w.nodes[i]
		if math.Hypot(math.Hypot(p.state.X-n.X, p.state.Y-n.Y), p.state.Z) <= 72 && now.UnixMilli() >= n.ReadyAtMS {
			n.ReadyAtMS = now.Add(10 * time.Second).UnixMilli()
			p.state.Score++
			if p.quest.Status == "active" && p.quest.Progress < questGoal {
				p.quest.Progress++
			}
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
	state := &contracts.WorldState{SceneVersion: w.layout.Version, TimeMS: now.UnixMilli(), Width: width, Height: height, Players: make([]*contracts.WorldPlayer, 0, len(w.players)), Chats: make([]*contracts.WorldChat, 0, len(w.chats)), Nodes: make([]*contracts.WorldNode, 0, len(w.nodes))}
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
