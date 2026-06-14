package managementhttp

import (
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

type LoginFailureTracker struct {
	now func() time.Time

	mu      sync.Mutex
	entries map[string][]time.Time
}

type LoginFailureRecorder interface {
	IsLimited(string, int, time.Duration) bool
	RecordFailure(string, int, time.Duration)
	Reset(string)
}

func NewLoginFailureTracker(now func() time.Time) *LoginFailureTracker {
	if now == nil {
		now = time.Now
	}
	return &LoginFailureTracker{
		now:     now,
		entries: make(map[string][]time.Time),
	}
}

func (t *LoginFailureTracker) IsLimited(source string, limit int, window time.Duration) bool {
	if !loginFailureTrackingEnabled(source, limit, window) {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	entries := t.prunedLocked(source, window)
	return len(entries) >= limit
}

func (t *LoginFailureTracker) RecordFailure(source string, limit int, window time.Duration) {
	if !loginFailureTrackingEnabled(source, limit, window) {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	entries := append(t.prunedLocked(source, window), t.now().UTC())
	t.entries[source] = entries
}

func (t *LoginFailureTracker) Reset(source string) {
	if t == nil || source == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, source)
}

func (t *LoginFailureTracker) prunedLocked(source string, window time.Duration) []time.Time {
	if t == nil || source == "" {
		return nil
	}

	entries := t.entries[source]
	if len(entries) == 0 {
		delete(t.entries, source)
		return nil
	}

	cutoff := t.now().UTC().Add(-window)
	filtered := entries[:0]
	for _, entry := range entries {
		if !entry.Before(cutoff) {
			filtered = append(filtered, entry)
		}
	}

	if len(filtered) == 0 {
		delete(t.entries, source)
		return nil
	}

	t.entries[source] = filtered
	return filtered
}

func loginFailureTrackingEnabled(source string, limit int, window time.Duration) bool {
	return source != "" && limit > 0 && window > 0
}

func LoginFailureLimit(cfg config.Config) int {
	return cfg.Admin.LoginFailLimit
}

func LoginFailureWindow(cfg config.Config) time.Duration {
	seconds := cfg.Admin.LoginFailWindowSecs
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}
