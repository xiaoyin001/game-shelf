// 文件说明：以显式单测存储验证探索状态、错误恢复和锁边界；真实MongoDB另由集成测试覆盖。
package world

import (
	"context"
	"errors"
	"game-shelf/pkg/contracts"
	"game-shelf/pkg/fieldbook"
	"sync"
	"testing"
	"time"
)

type memoryBooks struct {
	mu                          sync.Mutex
	data                        map[string]*contracts.Journal
	loadErr, saveErr, uncertain bool
	block                       chan struct{}
	started                     chan struct{}
}

func newMemoryBooks() *memoryBooks { return &memoryBooks{data: map[string]*contracts.Journal{}} }
func (s *memoryBooks) Load(ctx context.Context, id string) (*contracts.Journal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr {
		return nil, errors.New("load failed")
	}
	return fieldbook.Clone(s.data[id]), ctx.Err()
}
func (s *memoryBooks) Save(ctx context.Context, id string, next *contracts.Journal) (*contracts.Journal, error) {
	if s.started != nil {
		s.started <- struct{}{}
	}
	if s.block != nil {
		select {
		case <-s.block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveErr {
		return nil, errors.New("save failed")
	}
	old := fieldbook.Clone(s.data[id])
	if old.Revision != next.Revision {
		return nil, errors.New("conflict")
	}
	copy := fieldbook.Clone(next)
	copy.Revision++
	s.data[id] = copy
	if s.uncertain {
		return nil, errors.New("acknowledgment lost")
	}
	return fieldbook.Clone(copy), nil
}
func bookPosition(w *World, id string, x, y, z float64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[id]
	p.state.X, p.state.Y, p.state.Z, p.state.Grounded, p.vz = x, y, z, true, 0
}
func trailChoice(w *World, id, session string, now time.Time) *contracts.WorldResp {
	return w.Apply(&contracts.WorldReq{UserID: id, SessionID: session, Action: "choose", NPCID: "keeper", ChoiceID: "trail"}, now)
}
func TestFieldBookFullStoriesAndPersistence(t *testing.T) {
	now := time.Unix(1000, 0)
	store := newMemoryBooks()
	w := NewPersistent(store)
	enter(w, "a", "a", now)
	enter(w, "b", "b", now)
	if r := trailChoice(w, "a", "a", now); r.Error != "npc_out_of_range" {
		t.Fatal(r)
	}
	for index, entry := range fieldbook.Entries() {
		bookPosition(w, "a", 560, 448, 0)
		if r := trailChoice(w, "a", "a", now); r.Error != "" || r.Book.Trail.ID != entry.ID {
			t.Fatal(r)
		}
		if r := command(w, "a", "a", "explore", now); r.Error != "clue_out_of_range" {
			t.Fatal("remote investigation", r)
		}
		for step, target := range entry.Stops {
			bookPosition(w, "a", target.X, target.Y, target.Z)
			if entry.ID == "flute" && step == 1 {
				bookPosition(w, "a", target.X+43, target.Y, 0)
				if r := command(w, "a", "a", "explore", now); r.Error != "clue_out_of_range" {
					t.Fatal("ground can reach high clue", r)
				}
				bookPosition(w, "a", target.X, target.Y, target.Z)
				w.players["a"].state.Grounded = false
				if r := command(w, "a", "a", "explore", now); r.Error != "clue_out_of_range" {
					t.Fatal("airborne accepted", r)
				}
				w.players["a"].state.Grounded = true
			}
			r := command(w, "a", "a", "explore", now)
			if r.Error != "" {
				t.Fatal(entry.ID, step, r.Error)
			}
			if step == 0 {
				if r.Book.Trail.Step != 2 {
					t.Fatal(r)
				}
				// 中途离开、换会话与新World均从存储恢复，不退回第一步。
				command(w, "a", "a", "leave", now)
				w = NewPersistent(store)
				if restored := enter(w, "a", "a", now); restored.Book.Trail.Step != 2 {
					t.Fatal(restored)
				}
			} else if !r.Book.Collections[index].Found || r.Book.Collections[index].Story == "" || r.Book.Trail != nil {
				t.Fatal(r)
			}
		}
	}
	if r := command(w, "a", "a", "explore", now); r.Error != "no_active_trail" {
		t.Fatal(r)
	}
	if r := enter(w, "a", "new", now); len(r.Book.Collections) != 3 || !r.Book.Collections[2].Found {
		t.Fatal(r)
	}
	if r := command(w, "a", "a", "leave", now); r.Error == "" {
		t.Fatal("old session deleted restored player")
	}
	peer := enter(w, "b", "b", now)
	if peer.Book.Collections[0].Found || peer.Book.Trail != nil {
		t.Fatal("private state leaked")
	}
	peer.Book.Collections[0].Found = true
	if command(w, "b", "b", "poll", now).Book.Collections[0].Found {
		t.Fatal("response mutated state")
	}
}
func TestFieldBookConcurrentUniqueAndFailures(t *testing.T) {
	now := time.Unix(1000, 0)
	store := newMemoryBooks()
	w := NewPersistent(store)
	store.loadErr = true
	if enter(w, "a", "a", now).Error != "progress_unavailable" || len(w.players) != 0 {
		t.Fatal("failed read became empty save")
	}
	store.loadErr = false
	enter(w, "a", "a", now)
	bookPosition(w, "a", 560, 448, 0)
	store.saveErr = true
	if trailChoice(w, "a", "a", now).Error != "progress_unavailable" || w.players["a"].journal.Active != "" {
		t.Fatal("uncommitted progress announced")
	}
	store.saveErr = false
	store.uncertain = true
	if trailChoice(w, "a", "a", now).Error != "progress_unavailable" || w.players["a"].journal.Active != "bell" {
		t.Fatal("ambiguous write not reloaded")
	}
	store.uncertain = false
	bookPosition(w, "a", 600, 520, 0)
	if command(w, "a", "a", "explore", now).Error != "" {
		t.Fatal("first stop")
	}
	bookPosition(w, "a", 628, 448, 8)
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() { defer wg.Done(); command(w, "a", "a", "explore", now) }()
	}
	wg.Wait()
	saved, _ := store.Load(context.Background(), "a")
	if len(saved.Found) != 1 || saved.Revision != 3 {
		t.Fatal("duplicate reward", saved)
	}
}
func TestFieldBookIOReleasesWorldLockAndSessionExpires(t *testing.T) {
	now := time.Unix(1000, 0)
	store := newMemoryBooks()
	w := NewPersistent(store)
	enter(w, "a", "a", now)
	enter(w, "b", "b", now)
	bookPosition(w, "a", 560, 448, 0)
	store.started = make(chan struct{}, 1)
	store.block = make(chan struct{})
	done := make(chan *contracts.WorldResp, 1)
	go func() { done <- trailChoice(w, "a", "a", now) }()
	<-store.started
	other := make(chan *contracts.WorldResp, 1)
	go func() { other <- command(w, "b", "b", "poll", now.Add(6*time.Second)) }()
	select {
	case <-other:
	case <-time.After(time.Second):
		t.Fatal("storage blocked whole world")
	}
	close(store.block)
	if r := <-done; r.Error != "session_replaced_or_expired" {
		t.Fatal("expired request was announced", r)
	}
	store.block = nil
	store.started = nil
	restored := enter(w, "a", "new", now.Add(6*time.Second))
	if restored.Book.Trail == nil || restored.Book.Trail.ID != "bell" {
		t.Fatal("committed data lost after expiration", restored)
	}
}
