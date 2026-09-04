package logging

import (
	"sync"
	"time"
)

// FailureTracker aggregates repeated background failures without changing job counters.
// Callers supply stable operation scopes and machine-readable causes, never log text.
type FailureTracker struct {
	mu    sync.Mutex
	items map[string]failureState
}

type failureState struct {
	cause                string
	first, last, emitted time.Time
	count, pending       int
}

const failureSummaryInterval = 5 * time.Minute
const failureScopeLimit = 1024

// Failure returns the number of occurrences represented by a new log record.
// Zero means the occurrence is retained for the next summary or recovery.
func (tracker *FailureTracker) Failure(scope, cause string, now time.Time) int {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if tracker.items == nil {
		tracker.items = make(map[string]failureState)
	}
	state, found := tracker.items[scope]
	if !found || state.cause != cause {
		if !found && len(tracker.items) >= failureScopeLimit {
			var oldest string
			var at time.Time
			for key, entry := range tracker.items {
				if oldest == "" || entry.last.Before(at) {
					oldest, at = key, entry.last
				}
			}
			delete(tracker.items, oldest)
		}
		tracker.items[scope] = failureState{cause: cause, first: now, last: now, emitted: now, count: 1}
		return 1
	}
	state.count++
	state.pending++
	state.last = now
	count := 0
	if now.Sub(state.emitted) >= failureSummaryInterval {
		count, state.pending, state.emitted = state.pending, 0, now
	}
	tracker.items[scope] = state
	return count
}

// Recover ends an observed failure episode and returns its total occurrence count.
func (tracker *FailureTracker) Recover(scope string) int {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	state, found := tracker.items[scope]
	if !found {
		return 0
	}
	delete(tracker.items, scope)
	return state.count
}
