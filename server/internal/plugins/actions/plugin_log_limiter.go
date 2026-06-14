package actions

import (
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
)

type PluginLogLimiter struct {
	mu      sync.Mutex
	now     func() time.Time
	limit   permission.RateLimit
	records map[string][]time.Time
}

func NewPluginLogLimiter(cfg config.Config) *PluginLogLimiter {
	return &PluginLogLimiter{
		now:     time.Now,
		limit:   parsePluginLogRateLimit(cfg),
		records: make(map[string][]time.Time),
	}
}

func (l *PluginLogLimiter) ApplyConfig(cfg config.Config) {
	if l == nil {
		return
	}
	l.SetLimit(parsePluginLogRateLimit(cfg))
}

func (l *PluginLogLimiter) SetLimit(limit permission.RateLimit) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.limit = limit
	if len(l.records) == 0 {
		return
	}
	now := l.now().UTC()
	for pluginID, entries := range l.records {
		l.records[pluginID] = prunePluginLogEntries(entries, now, l.limit.Window)
		if len(l.records[pluginID]) == 0 {
			delete(l.records, pluginID)
		}
	}
}

func (l *PluginLogLimiter) Allow(pluginID string) bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now().UTC()
	entries := prunePluginLogEntries(l.records[pluginID], now, l.limit.Window)
	if len(entries) >= l.limit.Count {
		l.records[pluginID] = entries
		return false
	}
	l.records[pluginID] = append(entries, now)
	return true
}

func prunePluginLogEntries(entries []time.Time, now time.Time, window time.Duration) []time.Time {
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

func parsePluginLogRateLimit(cfg config.Config) permission.RateLimit {
	limit, err := permission.ParseRateLimit(strings.TrimSpace(cfg.Log.RateLimitPerPlugin))
	if err == nil {
		return limit
	}
	limit, _ = permission.ParseRateLimit(defaultPluginLogRateLimit)
	return limit
}
