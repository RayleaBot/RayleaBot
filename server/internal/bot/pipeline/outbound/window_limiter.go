package outbound

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

type windowLimiter struct {
	mu      sync.Mutex
	now     func() time.Time
	limit   config.RateLimit
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

func newWindowLimiter(now func() time.Time, limit config.RateLimit) *windowLimiter {
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
	if index == 0 {
		return entries
	}
	// Release burst-sized storage after expiry or a lower configured limit.
	// Small, frequently reused windows keep their capacity without allocating.
	if cap(entries) > 64 && len(entries)-index < cap(entries)/4 {
		return append([]time.Time(nil), entries[index:]...)
	}
	remaining := copy(entries, entries[index:])
	clear(entries[remaining:])
	return entries[:remaining]
}

func (l *windowLimiter) SetLimit(limit config.RateLimit) {
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
	if strings.TrimSpace(key) == "" {
		return nil
	}

	waiter, err := l.reserveOrEnqueue(ctx, key)
	if err != nil || waiter == nil {
		return err
	}
	select {
	case <-waiter.ready:
	case <-ctx.Done():
		l.cancelWaiter(key, waiter)
		return ctx.Err()
	}

	for {
		waitFor, updated, err := l.tryReserve(ctx, key, waiter)
		if err != nil {
			l.cancelWaiter(key, waiter)
			return err
		}
		if waitFor == 0 {
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

func (l *windowLimiter) reserveOrEnqueue(ctx context.Context, key string) (*windowWaiter, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	state := l.windows[key]
	if state == nil {
		state = &windowState{}
		l.windows[key] = state
	}
	wasEmpty := len(state.queue) == 0
	// Admission and the queue check share one lock so new arrivals cannot
	// bypass an older waiter when a window expires or the limit changes.
	if wasEmpty {
		now := l.now().UTC()
		state.records = pruneWindowRecords(state.records, now, l.limit.Window)
		if len(state.records) < l.limit.Count {
			state.records = append(state.records, now)
			return nil, nil
		}
	}
	waiter := &windowWaiter{ready: make(chan struct{})}
	state.queue = append(state.queue, waiter)
	if wasEmpty {
		close(waiter.ready)
	}
	return waiter, nil
}

func (l *windowLimiter) tryReserve(ctx context.Context, key string, waiter *windowWaiter) (time.Duration, <-chan struct{}, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return 0, nil, err
	}
	state := l.windows[key]
	if state == nil || len(state.queue) == 0 || state.queue[0] != waiter {
		return time.Millisecond, l.updated, nil
	}

	now := l.now().UTC()
	state.records = pruneWindowRecords(state.records, now, l.limit.Window)
	if len(state.records) < l.limit.Count {
		state.records = append(state.records, now)
		l.popHead(key, state)
		return 0, l.updated, nil
	}

	waitUntil := state.records[0].Add(l.limit.Window)
	waitFor := time.Until(waitUntil)
	if waitFor <= 0 {
		waitFor = time.Millisecond
	}
	return waitFor, l.updated, nil
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
		copy(state.queue[index:], state.queue[index+1:])
		state.queue[len(state.queue)-1] = nil
		state.queue = state.queue[:len(state.queue)-1]
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
		state.queue[0] = nil
		if len(state.queue) == 1 {
			state.queue = state.queue[:0]
		} else {
			state.queue = state.queue[1:]
		}
	}
	if len(state.queue) > 0 {
		close(state.queue[0].ready)
		return
	}
	if len(state.records) == 0 {
		delete(l.windows, key)
	}
}
