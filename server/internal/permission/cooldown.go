package permission

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RateLimit struct {
	Count  int
	Window time.Duration
}

type CooldownTracker struct {
	userLimit  RateLimit
	groupLimit RateLimit
	mu         sync.Mutex
	windows    map[string]*slidingWindow
}

type slidingWindow struct {
	timestamps []time.Time
	limit      RateLimit
}

func NewCooldownTracker(userLimit, groupLimit RateLimit) *CooldownTracker {
	return &CooldownTracker{
		userLimit:  userLimit,
		groupLimit: groupLimit,
		windows:    make(map[string]*slidingWindow),
	}
}

func ParseRateLimit(raw string) (RateLimit, error) {
	raw = strings.TrimSpace(raw)
	countText, windowText, ok := strings.Cut(raw, "/")
	if !ok {
		return RateLimit{}, fmt.Errorf("invalid rate limit format %q", raw)
	}

	count, err := strconv.Atoi(strings.TrimSpace(countText))
	if err != nil || count <= 0 {
		return RateLimit{}, fmt.Errorf("invalid rate limit count %q", countText)
	}

	window, err := time.ParseDuration(strings.TrimSpace(windowText))
	if err != nil || window <= 0 {
		return RateLimit{}, fmt.Errorf("invalid rate limit window %q", windowText)
	}

	return RateLimit{
		Count:  count,
		Window: window,
	}, nil
}

func (t *CooldownTracker) Allow(key string) bool {
	if t == nil {
		return true
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	w, ok := t.windows[key]
	if !ok {
		limit := t.limitForKey(key)
		w = &slidingWindow{limit: limit}
		t.windows[key] = w
	}

	// Evict expired timestamps.
	cutoff := now.Add(-w.limit.Window)
	valid := 0
	for _, ts := range w.timestamps {
		if ts.After(cutoff) {
			w.timestamps[valid] = ts
			valid++
		}
	}
	w.timestamps = w.timestamps[:valid]

	if len(w.timestamps) >= w.limit.Count {
		return false
	}

	w.timestamps = append(w.timestamps, now)
	return true
}

func (t *CooldownTracker) limitForKey(key string) RateLimit {
	if len(key) > 6 && key[:6] == "group:" {
		return t.groupLimit
	}
	return t.userLimit
}

// Cleanup removes expired windows. Call periodically.
func (t *CooldownTracker) Cleanup() {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	for key, w := range t.windows {
		cutoff := now.Add(-w.limit.Window)
		hasValid := false
		for _, ts := range w.timestamps {
			if ts.After(cutoff) {
				hasValid = true
				break
			}
		}
		if !hasValid {
			delete(t.windows, key)
		}
	}
}
