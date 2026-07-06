package outbound

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/permission"
)

type windowLimiter struct {
	mu      sync.Mutex
	now     func() time.Time
	limit   permission.RateLimit
	updated chan struct{}
	windows map[string]*windowState
}

type windowState struct {
	queue   []*windowWaiter
	records []time.Time
}

type windowWaiter struct {
	ready chan struct{}
}

func newWindowLimiter(now func() time.Time, limit permission.RateLimit) *windowLimiter {
	if now == nil {
		now = time.Now
	}
	return &windowLimiter{
		now:     now,
		limit:   limit,
		updated: make(chan struct{}),
		windows: make(map[string]*windowState),
	}
}

func pruneWindowRecords(entries []time.Time, now time.Time, window time.Duration) []time.Time {
	if window <= 0 {
		return nil
	}
	cutoff := now.Add(-window)
	index := 0
	for index < len(entries) && entries[index].Before(cutoff) {
		index++
	}
	return append([]time.Time(nil), entries[index:]...)
}

func (l *windowLimiter) SetLimit(limit permission.RateLimit) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.limit = limit
	close(l.updated)
	l.updated = make(chan struct{})
	now := l.now().UTC()
	for key, state := range l.windows {
		state.records = pruneWindowRecords(state.records, now, l.limit.Window)
		if len(state.records) == 0 && len(state.queue) == 0 {
			delete(l.windows, key)
		}
	}
}

func (l *windowLimiter) Wait(ctx context.Context, key string) error {
	if l == nil || strings.TrimSpace(key) == "" {
		return nil
	}

	waiter := l.enqueue(key)
	select {
	case <-waiter.ready:
	case <-ctx.Done():
		l.cancelWaiter(key, waiter)
		return ctx.Err()
	}

	for {
		waitFor, updated, ok := l.tryReserve(key, waiter)
		if ok {
			return nil
		}

		timer := time.NewTimer(waitFor)
		select {
		case <-timer.C:
		case <-updated:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			l.cancelWaiter(key, waiter)
			return ctx.Err()
		}
	}
}

func (l *windowLimiter) enqueue(key string) *windowWaiter {
	waiter := &windowWaiter{ready: make(chan struct{})}

	l.mu.Lock()
	defer l.mu.Unlock()

	state := l.windows[key]
	if state == nil {
		state = &windowState{}
		l.windows[key] = state
	}
	wasEmpty := len(state.queue) == 0
	state.queue = append(state.queue, waiter)
	if wasEmpty {
		close(waiter.ready)
	}
	return waiter
}

func (l *windowLimiter) tryReserve(key string, waiter *windowWaiter) (time.Duration, <-chan struct{}, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	state := l.windows[key]
	if state == nil || len(state.queue) == 0 || state.queue[0] != waiter {
		return time.Millisecond, l.updated, false
	}

	now := l.now().UTC()
	state.records = pruneWindowRecords(state.records, now, l.limit.Window)
	if len(state.records) < l.limit.Count {
		state.records = append(state.records, now)
		l.popHead(key, state)
		return 0, l.updated, true
	}

	waitUntil := state.records[0].Add(l.limit.Window)
	waitFor := time.Until(waitUntil)
	if waitFor <= 0 {
		waitFor = time.Millisecond
	}
	return waitFor, l.updated, false
}

func (l *windowLimiter) cancelWaiter(key string, waiter *windowWaiter) {
	l.mu.Lock()
	defer l.mu.Unlock()

	state := l.windows[key]
	if state == nil {
		return
	}
	for index, candidate := range state.queue {
		if candidate != waiter {
			continue
		}
		state.queue = append(state.queue[:index], state.queue[index+1:]...)
		if index == 0 && len(state.queue) > 0 {
			close(state.queue[0].ready)
		}
		break
	}
	now := l.now().UTC()
	state.records = pruneWindowRecords(state.records, now, l.limit.Window)
	if len(state.records) == 0 && len(state.queue) == 0 {
		delete(l.windows, key)
	}
}

func (l *windowLimiter) popHead(key string, state *windowState) {
	if len(state.queue) > 0 {
		state.queue = state.queue[1:]
	}
	if len(state.queue) > 0 {
		close(state.queue[0].ready)
		return
	}
	if len(state.records) == 0 {
		delete(l.windows, key)
	}
}
