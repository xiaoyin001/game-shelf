// 文件说明：在世界锁内管理房间归属、双方准备、固定名册和20秒断线保留。
package world

import (
	"crypto/rand"
	"encoding/hex"
	"game-shelf/internal/challenge"
	"game-shelf/pkg/contracts"
	"strings"
	"time"
)

const reconnectGrace = 20 * time.Second
const codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

type roomMember struct {
	id     string
	name   string
	ready  bool
	online bool
	quit   bool
	until  time.Time
	pose   contracts.WorldPlayer
}
type room struct {
	id        string
	code      string
	mode      string
	host      string
	members   []*roomMember
	round     *challenge.Round
	practice  string
	qualified bool
	result    *resultJob
}

func (r *room) phase() string {
	if r.round == nil {
		return "waiting"
	}
	return r.round.Phase()
}
func (r *room) member(id string) *roomMember {
	for _, m := range r.members {
		if m.id == id {
			return m
		}
	}
	return nil
}
func (r *room) eligibility() string {
	if r.qualified {
		return "eligible"
	}
	if r.practice != "" {
		return "practice"
	}
	for _, m := range r.members {
		if !m.online && !m.quit {
			return "reconnecting"
		}
	}
	return "eligible"
}
func (r *room) snapshot(now time.Time) *contracts.ChallengeRoom {
	s := &contracts.ChallengeRoom{ID: r.id, Code: r.code, Mode: r.mode, HostID: r.host, Phase: r.phase(), Eligibility: r.eligibility(), PracticeReason: r.practice, Members: make([]*contracts.RoomMember, 0, len(r.members))}
	for _, m := range r.members {
		remaining := int64(0)
		if !m.online && !m.quit && !m.until.IsZero() {
			remaining = max(0, m.until.Sub(now).Milliseconds())
		}
		s.Members = append(s.Members, &contracts.RoomMember{ID: m.id, Name: m.name, Ready: m.ready, Online: m.online, Quit: m.quit, ReconnectMS: remaining})
	}
	return s
}
func randomID() string {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(token[:])
}
func (w *World) inviteCode() string {
	for {
		var token [6]byte
		if _, err := rand.Read(token[:]); err != nil {
			panic(err)
		}
		var b [6]byte
		for i, v := range token {
			b[i] = codeAlphabet[int(v)%len(codeAlphabet)]
		}
		code := string(b[:])
		if w.codes[code] == "" {
			return code
		}
	}
}
func (w *World) clearRoomChat(id string) {
	kept := w.chats[:0]
	for _, c := range w.chats {
		if c.scope != id {
			kept = append(kept, c)
		}
	}
	w.chats = kept
}
func (w *World) removeRoom(r *room) {
	for _, m := range r.members {
		if w.memberships[m.id] == r.id {
			delete(w.memberships, m.id)
		}
		if p := w.players[m.id]; p != nil && p.roomID == r.id {
			p.roomID = ""
		}
	}
	delete(w.rooms, r.id)
	if r.code != "" {
		delete(w.codes, r.code)
	}
	w.clearRoomChat(r.id)
}
func (w *World) resetReady(r *room) {
	for _, m := range r.members {
		m.ready = false
	}
}
func (w *World) transferHost(r *room) {
	if h := r.member(r.host); h != nil && !h.quit {
		return
	}
	for _, m := range r.members {
		if !m.quit {
			r.host = m.id
			return
		}
	}
}
func (w *World) abandonMember(r *room, m *roomMember, reason string) {
	if m.quit {
		return
	}
	invalidate := r.phase() == "running" || (r.phase() == "finished" && !m.online && r.eligibility() == "reconnecting")
	m.quit = true
	m.online = false
	m.ready = false
	m.until = time.Time{}
	if w.memberships[m.id] == r.id {
		delete(w.memberships, m.id)
	}
	if r.round != nil && invalidate {
		if r.practice == "" {
			r.practice = reason
		}
		if r.phase() == "running" {
			r.round.ReleaseHeld(m.id)
		}
	}
	w.transferHost(r)
}
func (w *World) cleanMembers(r *room) {
	kept := r.members[:0]
	for _, m := range r.members {
		if !m.quit {
			kept = append(kept, m)
		}
	}
	r.members = kept
}
func (w *World) disconnect(p *player, now time.Time) {
	if r := w.rooms[p.roomID]; r != nil {
		if r.mode == "solo" {
			r.round.Advance(now)
			w.finalize(r)
			r.round.Abandon()
			w.removeRoom(r)
		} else if m := r.member(p.state.ID); m != nil && !m.quit {
			m.pose = p.state
			m.online = false
			m.until = now.Add(reconnectGrace)
			if r.phase() == "waiting" {
				w.resetReady(r)
			}
		}
	}
	delete(w.players, p.state.ID)
}

// Tick 不依赖客户端轮询；到时、离线超期与身体租约在同一锁内推进。
func (w *World) Tick(now time.Time) { w.mu.Lock(); defer w.mu.Unlock(); w.tick(now) }
func (w *World) tick(now time.Time) {
	for _, p := range w.players {
		if now.Sub(p.lastSeen) > playerTTL {
			w.disconnect(p, now)
		}
	}
	w.advance(now)
	for _, r := range w.rooms {
		for _, m := range r.members {
			if !m.online && !m.quit && !m.until.IsZero() && !now.Before(m.until) {
				// 先判断线期限，再结束局，不能在同一刻误确认为有效成绩。
				w.abandonMember(r, m, "reconnect_expired")
			}
		}
		if r.round != nil {
			r.round.Advance(now)
			w.finalize(r)
		}
		if r.phase() == "waiting" {
			w.cleanMembers(r)
		}
		active := 0
		for _, m := range r.members {
			if !m.quit {
				active++
			}
		}
		if active == 0 {
			w.removeRoom(r)
		}
	}
}
func (w *World) restoreRoom(p *player, now time.Time) bool {
	r := w.rooms[w.memberships[p.state.ID]]
	if r == nil {
		return false
	}
	m := r.member(p.state.ID)
	if m == nil || m.quit {
		delete(w.memberships, p.state.ID)
		return false
	}
	p.roomID = r.id
	p.state = m.pose
	p.state.Name = m.name
	if w.blocked(p, p.state.X, p.state.Y) {
		x, y, ok := w.spawnIn(p.state.ID, r.id, m.pose.X, m.pose.Y)
		if !ok {
			p.roomID = ""
			return false
		}
		p.state.X, p.state.Y, p.state.Z = x, y, 0
		p.state.Grounded = true
	} else {
		p.state.Grounded = p.state.Grounded && w.supported(p)
	}
	m.online = true
	m.until = time.Time{}
	m.pose = p.state
	w.finalize(r)
	return true
}
func (w *World) placeForRoom(p *player, r *room, slot int, now time.Time) {
	y := w.layout.Workshop.SpawnY
	if r.mode == "duo" {
		y += float64(slot*44 - 22)
	}
	p.roomID = r.id
	p.state.Z = 0
	x := w.layout.Workshop.SpawnX
	if w.blocked(p, x, y) {
		if sx, sy, ok := w.spawnIn(p.state.ID, r.id, x, y); ok {
			x, y = sx, sy
		}
	}
	p.state.X, p.state.Y = x, y
	p.state.Grounded = true
	p.vz = 0
	p.lastMove = now
}
func (w *World) newRoom(p *player, mode string, now time.Time) string {
	if p.roomID != "" {
		return "already_in_room"
	}
	if mode == "solo" && len(w.results) >= 100 {
		return "results_busy"
	}
	if len(w.rooms) >= 100 {
		return "rooms_full"
	}
	r := &room{id: randomID(), mode: mode, host: p.state.ID, members: []*roomMember{{id: p.state.ID, name: p.state.Name, online: true}}}
	if mode == "duo" {
		r.code = w.inviteCode()
		w.codes[r.code] = r.id
	}
	w.rooms[r.id] = r
	w.memberships[p.state.ID] = r.id
	w.placeForRoom(p, r, 0, now)
	r.members[0].pose = p.state
	if mode == "solo" {
		r.round = challenge.New(p.state.ID, p.state.Name, now, w.layout.Workshop)
	}
	return ""
}
func (w *World) leaveRoom(p *player, roomID string, now time.Time) string {
	r := w.rooms[p.roomID]
	if r == nil {
		return ""
	}
	if r.mode == "duo" && roomID != r.id {
		return "stale_room"
	}
	x, y, ok := w.spawn(p.state.ID)
	if !ok {
		return "world_full"
	}
	if r.mode == "solo" {
		r.round.Abandon()
		w.removeRoom(r)
	} else {
		w.abandonMember(r, r.member(p.state.ID), "member_left")
		if r.phase() == "waiting" {
			w.cleanMembers(r)
			w.resetReady(r)
		}
		w.clearRoomChat(r.id)
		count := 0
		for _, m := range r.members {
			if !m.quit {
				count++
			}
		}
		if count == 0 {
			w.removeRoom(r)
		}
	}
	p.roomID = ""
	p.state.X, p.state.Y, p.state.Z = x, y, 0
	p.state.Grounded = true
	p.vz = 0
	p.lastMove = now
	return ""
}
func (w *World) roomAction(p *player, req *contracts.WorldReq, now time.Time) string {
	r := w.rooms[p.roomID]
	switch req.Action {
	case "start_solo":
		if len(w.results) >= 100 {
			return "results_busy"
		}
		if r != nil {
			if r.mode != "solo" {
				return "already_in_room"
			}
			if r.phase() == "running" {
				return "already_in_round"
			}
			w.removeRoom(r)
		}
		return w.newRoom(p, "solo", now)
	case "create_room":
		return w.newRoom(p, "duo", now)
	case "join_room":
		if r != nil {
			return "already_in_room"
		}
		code := strings.ToUpper(strings.TrimSpace(req.RoomCode))
		if len(code) != 6 {
			return "room_unavailable"
		}
		for _, c := range code {
			if !strings.ContainsRune(codeAlphabet, c) {
				return "room_unavailable"
			}
		}
		target := w.rooms[w.codes[code]]
		if target == nil {
			return "room_unavailable"
		}
		if target.phase() != "waiting" {
			return "room_locked"
		}
		if len(target.members) >= 2 {
			return "room_full"
		}
		target.members = append(target.members, &roomMember{id: p.state.ID, name: p.state.Name, online: true})
		w.memberships[p.state.ID] = target.id
		w.placeForRoom(p, target, len(target.members)-1, now)
		target.members[len(target.members)-1].pose = p.state
		w.resetReady(target)
		w.clearRoomChat(target.id)
		return ""
	case "leave_round":
		return w.leaveRoom(p, req.RoomID, now)
	}
	if r == nil || r.mode != "duo" {
		return "no_room"
	}
	if req.RoomID != r.id {
		return "stale_room"
	}
	m := r.member(p.state.ID)
	if m == nil || m.quit {
		return "not_a_member"
	}
	switch req.Action {
	case "set_ready":
		if r.phase() != "waiting" {
			return "room_locked"
		}
		m.ready = req.Ready
		return ""
	case "start_duo":
		if r.host != p.state.ID {
			return "host_only"
		}
		if r.phase() != "waiting" {
			return "room_locked"
		}
		if len(r.members) != 2 {
			return "both_ready_required"
		}
		for _, member := range r.members {
			if !member.ready || !member.online || member.quit {
				return "both_ready_required"
			}
		}
		if len(w.results) >= 100 {
			return "results_busy"
		}
		// 同锁内先移出旧作用域，避免准备区的站位改变固定开局出生点。
		for _, member := range r.members {
			w.players[member.id].roomID = ""
		}
		for slot, member := range r.members {
			w.placeForRoom(w.players[member.id], r, slot, now)
			member.pose = w.players[member.id].state
		}
		r.round = challenge.NewCoop(w.players[r.members[0].id].state, w.players[r.members[1].id].state, now, w.layout.Workshop)
		r.practice = ""
		w.clearRoomChat(r.id)
		return ""
	case "rematch":
		if r.host != p.state.ID {
			return "host_only"
		}
		if r.phase() != "finished" {
			return "round_not_finished"
		}
		if r.eligibility() == "reconnecting" {
			return "reconnect_pending"
		}
		r.round = nil
		r.practice = ""
		r.qualified = false
		r.result = nil
		w.cleanMembers(r)
		w.resetReady(r)
		w.clearRoomChat(r.id)
		return ""
	}
	return "unknown_action"
}
