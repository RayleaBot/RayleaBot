// Package conversation contains isolated A0 models, not a production registry.
package conversation

import (
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"testing"
)

type route struct{ protocol, adapter, bot, targetType, target, actor string }
type proposal struct {
	state   string
	expires int64
}
type session struct {
	phase                    string
	revision, turn, maxTurns int
	expires, absolute        int64
	state                    string
	next                     *proposal
	buffer                   []string
}
type registry struct {
	mu    sync.Mutex
	items map[route]*session
}

func (r *registry) register(key route, state map[string]any) *session {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.items == nil {
		r.items = make(map[route]*session)
	}
	if r.items[key] != nil {
		return nil
	}
	encoded, _ := json.Marshal(state)
	s := &session{phase: "registered", revision: 1, maxTurns: 3, expires: 60, absolute: 1800, state: string(encoded)}
	r.items[key] = s
	return s
}
func (r *registry) complete(key route, success bool, now int64) (replay []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.items[key]
	if s == nil {
		return nil
	}
	if !success || now >= s.absolute {
		if s.phase == "registered" {
			replay = slices.Clone(s.buffer)
		}
		delete(r.items, key)
		return replay
	}
	if s.phase == "registered" && now < s.expires {
		s.phase = "waiting"
		return nil
	}
	if s.phase == "handling" && s.next != nil && now < s.next.expires && s.turn < s.maxTurns {
		s.state, s.expires, s.next, s.phase = s.next.state, s.next.expires, nil, "waiting"
		s.revision++
		return nil
	}
	delete(r.items, key)
	return nil
}
func (r *registry) input(key route, message string, allowed, reserved bool, revision int, now int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.items[key]
	if s == nil || !allowed || !reserved || s.revision != revision || now >= s.expires {
		return false
	}
	if s.phase != "waiting" {
		if len(s.buffer) < 8 {
			s.buffer = append(s.buffer, message)
		}
		return false
	}
	s.phase = "claimed"
	return true
}
func (r *registry) buffered(key route) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.items[key]
	if s == nil || s.phase != "waiting" || len(s.buffer) == 0 {
		return ""
	}
	message := s.buffer[0]
	s.buffer = s.buffer[1:]
	return message
}
func (r *registry) start(key route, deliverable bool, now int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.items[key]
	if s == nil || s.phase != "claimed" {
		return false
	}
	if now >= s.expires {
		delete(r.items, key)
		return false
	}
	if !deliverable {
		s.phase = "waiting"
		return false
	}
	s.phase = "handling"
	s.turn++
	return true
}
func (r *registry) rewait(key route, revision int, state map[string]any, now int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.items[key]
	encoded, _ := json.Marshal(state)
	if s == nil || s.phase != "handling" || s.revision != revision || s.turn >= s.maxTurns || now >= s.absolute {
		return errors.New("stale")
	}
	if s.next != nil {
		if s.next.state == string(encoded) {
			return nil
		}
		return errors.New("stale")
	}
	s.next = &proposal{string(encoded), min(now+60, s.absolute)}
	return nil
}

func TestDesignSessionThreeTurnsAndBufferedFreshState(t *testing.T) {
	r := &registry{}
	key := route{"onebot11", "adapter", "bot", "group", "group", "actor"}
	state := map[string]any{"step": 0}
	s := r.register(key, state)
	state["step"] = 99
	if s.state != `{"step":0}` {
		t.Fatal("mutable state escaped registration")
	}
	r.input(key, "first", true, true, 1, 1)
	if s.phase != "registered" || s.turn != 0 {
		t.Fatal("registration consumed input before parent completion")
	}
	r.complete(key, true, 2)
	for turn := 1; turn <= 3; turn++ {
		if r.input(key, "denied", false, true, turn, 3) || r.input(key, "full", true, false, turn, 3) {
			t.Fatal("rejection claimed input")
		}
		message := r.buffered(key)
		if message == "" || !r.input(key, message, true, true, turn, 3) || !r.start(key, true, 3) || s.turn != turn {
			t.Fatal("turn did not start")
		}
		r.input(key, "next", true, true, turn, 4)
		if err := r.rewait(key, turn, map[string]any{"step": turn}, 4); turn < 3 && err != nil {
			t.Fatal(err)
		} else if turn == 3 && err == nil {
			t.Fatal("turn limit reset")
		}
		if turn < 3 {
			// The proposal must remain private until successful event completion.
			var value map[string]int
			_ = json.Unmarshal([]byte(s.state), &value)
			if value["step"] != turn-1 {
				t.Fatal("proposal committed too early")
			}
		}
		r.complete(key, true, 5)
		if turn < 3 {
			var value map[string]int
			_ = json.Unmarshal([]byte(s.state), &value)
			if s.revision != turn+1 || value["step"] != turn {
				t.Fatal("buffer would receive stale state")
			}
		}
	}
	if len(r.items) != 0 {
		t.Fatal("completed conversation retained occupancy")
	}
}

func TestDesignClaimRaceRollbackAndExpiry(t *testing.T) {
	r := &registry{}
	key := route{actor: "user"}
	s := r.register(key, nil)
	r.complete(key, true, 0)
	var wg sync.WaitGroup
	claimed := make(chan bool, 32)
	for range 32 {
		wg.Go(func() { claimed <- r.input(key, "reply", true, true, 1, 1) })
	}
	wg.Wait()
	close(claimed)
	winners := 0
	for ok := range claimed {
		if ok {
			winners++
		}
	}
	if winners != 1 || s.turn != 0 || len(s.buffer) != 8 {
		t.Fatal("claim not atomic or buffer unbounded")
	}
	if r.start(key, false, 1) || s.turn != 0 || s.phase != "waiting" {
		t.Fatal("failed delivery consumed turn")
	}
	if r.input(key, "late", true, true, 1, 60) {
		t.Fatal("exact deadline admitted")
	}
	r.complete(key, false, 60)
	if len(r.items) != 0 {
		t.Fatal("expiry retained route")
	}
}

func TestDesignClosureReplayOnlyBeforeRegistrationCommit(t *testing.T) {
	for _, phase := range []string{"registered", "waiting", "claimed", "handling"} {
		r := &registry{}
		key := route{actor: "actor"}
		s := r.register(key, nil)
		s.phase = phase
		s.buffer = []string{"a", "b"}
		replay := r.complete(key, false, 0)
		if (len(replay) == 2) != (phase == "registered") || len(r.items) != 0 {
			t.Fatalf("close %s replay=%v", phase, replay)
		}
		if len(r.complete(key, false, 0)) != 0 {
			t.Fatal("close repeated side effects")
		}
	}
}

func TestDesignIdentityComponentsDoNotCollide(t *testing.T) {
	r := &registry{}
	base := route{"onebot11", "adapter", "bot", "group", "target", "user"}
	keys := []route{base}
	for i := range 6 {
		key := base
		fields := []*string{&key.protocol, &key.adapter, &key.bot, &key.targetType, &key.target, &key.actor}
		*fields[i] += "-other"
		keys = append(keys, key)
	}
	for _, key := range keys {
		if r.register(key, nil) == nil {
			t.Fatal("cross-identity conflict")
		}
	}
	if r.register(base, nil) != nil {
		t.Fatal("same-scope conflict not enforced")
	}
}
