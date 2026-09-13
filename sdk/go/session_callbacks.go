package rayleabot

import (
	"context"
	"errors"
	"sync"
	"time"
)

const sessionCallbackLimit = 256

type sessionCallback struct {
	id, route string
	deadline  time.Time
	handler   HandlerFunc
	timer     *time.Timer
}
type sessionCallbacks struct {
	mu       sync.Mutex
	waiting  map[string]*sessionCallback
	routes   map[string]string
	owned    map[string]time.Time
	reserved int
	closed   bool
}

func (s *sessionCallbacks) reserve(id string, route ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return context.Canceled
	}
	if s.waiting == nil {
		s.waiting = map[string]*sessionCallback{}
		s.routes = map[string]string{}
		s.owned = map[string]time.Time{}
	}
	s.pruneLocked()
	_, known := s.owned[id]
	credit := 0
	if len(route) > 0 && s.routes[route[0]] != "" {
		credit = 1
	}
	if len(s.waiting)-credit+s.reserved >= sessionCallbackLimit || !known && len(s.owned)+s.reserved >= pendingActionLimit {
		return errors.New("rayleabot: pending conversation callback limit reached")
	}
	s.reserved++
	return nil
}
func (s *sessionCallbacks) release() { s.mu.Lock(); s.reserved--; s.mu.Unlock() }
func (s *sessionCallbacks) pruneLocked() {
	now := time.Now()
	for id, until := range s.owned {
		if !now.Before(until) {
			delete(s.owned, id)
		}
	}
}

func (s *sessionCallbacks) removeLocked(id string) {
	item := s.waiting[id]
	if item == nil {
		return
	}
	delete(s.waiting, id)
	if s.routes[item.route] == id {
		delete(s.routes, item.route)
	}
	if item.timer != nil {
		item.timer.Stop()
	}
}
func (s *sessionCallbacks) forget(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeLocked(id)
	s.pruneLocked()
}

// Direct registration hands the next input back to the ordinary handler.
func (s *sessionCallbacks) registered(ref SessionRef, route string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old := s.routes[route]; old != "" {
		s.removeLocked(old)
	}
	s.removeLocked(ref.SessionID)
	delete(s.owned, ref.SessionID)
}

func (s *sessionCallbacks) install(ref SessionRef, route string, deadline time.Time, handler HandlerFunc) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return context.Canceled
	}
	s.owned[ref.SessionID] = time.UnixMilli(ref.ExpiresAtMS).Add(retiredActionRetention)
	if !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	if old := s.routes[route]; old != "" {
		s.removeLocked(old)
	}
	s.removeLocked(ref.SessionID)
	item := &sessionCallback{id: ref.SessionID, route: route, deadline: deadline, handler: handler}
	s.waiting[item.id] = item
	s.routes[route] = item.id
	item.timer = time.AfterFunc(time.Until(deadline), func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.waiting[item.id] == item {
			s.removeLocked(item.id)
		}
	})
	return nil
}

// Claim on input receipt, before waiting for the regular execution permit.
func (s *sessionCallbacks) take(ref SessionRef) (HandlerFunc, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, true
	}
	s.pruneLocked()
	item := s.waiting[ref.SessionID]
	_, owned := s.owned[ref.SessionID]
	if item == nil {
		return nil, owned || time.Now().UnixMilli() >= ref.ExpiresAtMS
	}
	s.removeLocked(item.id)
	if !time.Now().Before(item.deadline) {
		return nil, true
	}
	return item.handler, true
}

func (s *sessionCallbacks) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	for id := range s.waiting {
		s.removeLocked(id)
	}
	clear(s.owned)
}
