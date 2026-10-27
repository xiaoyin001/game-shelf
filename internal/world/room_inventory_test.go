// 文件说明：补充独立三格、跨成员连击、等待名册变更和截止后的重连资格边界。
package world

import (
	"game-shelf/pkg/contracts"
	"testing"
	"time"
)

func TestTwoIndependentHandsAndCrossMemberCombo(t *testing.T) {
	w, n, r := duo(t)
	tops := []*contracts.ChallengeItem{}
	for _, i := range r.Challenge.Items {
		if i.Z > 0 {
			tops = append(tops, i)
		}
	}
	for member, id := range []string{"a", "b"} {
		for k := 0; k < 3; k++ {
			i := tops[member*3+k]
			p := w.players[id]
			p.state.X, p.state.Y = i.X, i.Y
			if s := w.Apply(&contracts.WorldReq{UserID: id, SessionID: id, Action: "pick", RoundID: r.Challenge.ID, ItemID: i.ID}, n); s.Error != "" {
				t.Fatal(s)
			}
		}
		p := w.players[id]
		p.state.X, p.state.Y = tops[6].X, tops[6].Y
		if s := w.Apply(&contracts.WorldReq{UserID: id, SessionID: id, Action: "pick", RoundID: r.Challenge.ID, ItemID: tops[6].ID}, n); s.Error != "hand_full" {
			t.Fatal("not individual capacity", s)
		}
	}
	state := command(w, "a", "a", "poll", n).Challenge
	held := 0
	for _, i := range state.Items {
		if i.Place == "hand" {
			held++
		}
	}
	if held != 6 {
		t.Fatal("not six independent held items", held)
	}
	for _, step := range []struct {
		id    string
		index int
	}{{"a", 0}, {"a", 1}, {"b", 4}, {"b", 5}} {
		p := w.players[step.id]
		p.state.X, p.state.Y = 700, 350
		if s := w.Apply(&contracts.WorldReq{UserID: step.id, SessionID: step.id, Action: "put", RoundID: r.Challenge.ID, ItemID: tops[step.index].ID}, n); s.Error != "" {
			t.Fatal(s)
		}
	}
	state = command(w, "b", "b", "poll", n).Challenge
	if state.Score != 250 || state.Multiplier != 15 || state.Members[0].Pairs != 1 || state.Members[1].Pairs != 1 {
		t.Fatal("combo or contribution not shared", state)
	}
}
func TestWaitingDisconnectResetsReadyAndReplacementDoesNotOverlap(t *testing.T) {
	n := time.Unix(1000, 0)
	w := New()
	for _, id := range []string{"a", "b", "c"} {
		enter(w, id, id, n)
	}
	created := roomReq(w, "a", "create_room", "", n, false)
	w.Apply(&contracts.WorldReq{UserID: "b", SessionID: "b", Action: "join_room", RoomCode: created.Room.Code}, n)
	roomReq(w, "a", "set_ready", created.Room.ID, n, true)
	roomReq(w, "b", "set_ready", created.Room.ID, n, true)
	command(w, "b", "b", "leave", n)
	r := command(w, "a", "a", "poll", n).Room
	if r.Members[0].Ready || r.Members[1].Ready {
		t.Fatal("disconnect retained readiness")
	}
	enter(w, "b", "new", n)
	roomReq(w, "a", "leave_round", r.ID, n, false)
	joined := w.Apply(&contracts.WorldReq{UserID: "c", SessionID: "c", Action: "join_room", RoomCode: r.Code}, n)
	if joined.Error != "" || joined.Room.HostID != "b" {
		t.Fatal("replacement waiting member failed", joined)
	}
	if w.blocked(w.players["b"], w.players["b"].state.X, w.players["b"].state.Y) || w.blocked(w.players["c"], w.players["c"].state.X, w.players["c"].state.Y) {
		t.Fatal("waiting join overlapped remaining host")
	}
}
func TestFinishedOfflineExpiryInvalidatesButFinishedDepartureDoesNot(t *testing.T) {
	w, n, r := duo(t)
	w.players["a"].lastSeen = n.Add(200 * time.Second)
	w.players["b"].lastSeen = n.Add(200 * time.Second)
	command(w, "b", "b", "leave", n.Add(175*time.Second))
	w.Tick(n.Add(180 * time.Second))
	if w.rooms[r.Room.ID].eligibility() != "reconnecting" {
		t.Fatal("finished without pending reconnect")
	}
	w.Tick(n.Add(195 * time.Second))
	if w.rooms[r.Room.ID].eligibility() != "practice" {
		t.Fatal("offline finished result became eligible at expiry")
	}
	w, n, r = duo(t)
	for _, p := range w.players {
		p.lastSeen = n.Add(200 * time.Second)
	}
	w.Tick(n.Add(180 * time.Second))
	roomReq(w, "b", "leave_round", r.Room.ID, n.Add(180*time.Second), false)
	if w.rooms[r.Room.ID].eligibility() != "eligible" {
		t.Fatal("departure retroactively invalidated finished result")
	}
}
