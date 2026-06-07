package localaction

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginfile"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginkv"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
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

func currentKVLimits(cfg config.Config) pluginkv.Limits {
	valueLimit := cfg.Storage.KVValueMaxBytes
	if valueLimit <= 0 {
		valueLimit = defaultKVValueMaxBytes
	}
	totalLimitMB := cfg.Storage.KVTotalLimitMB
	if totalLimitMB <= 0 {
		totalLimitMB = defaultKVTotalLimitMegabyte
	}
	return pluginkv.Limits{
		ValueMaxBytes: valueLimit,
		TotalMaxBytes: totalLimitMB * 1024 * 1024,
	}
}

func currentFileLimits(cfg config.Config) pluginfile.Limits {
	fileLimit := cfg.Storage.FileMaxBytes
	if fileLimit <= 0 {
		fileLimit = defaultFileMaxBytes
	}
	totalLimitMB := cfg.Storage.PluginWorkDirMB
	if totalLimitMB <= 0 {
		totalLimitMB = defaultPluginWorkdirMB
	}
	return pluginfile.Limits{
		FileMaxBytes:  fileLimit,
		TotalMaxBytes: totalLimitMB * 1024 * 1024,
	}
}

func redactValue(redactText func(string) string, value any) any {
	switch typed := value.(type) {
	case string:
		if redactText == nil {
			return typed
		}
		return redactText(typed)
	case []any:
		result := make([]any, len(typed))
		for index := range typed {
			result[index] = redactValue(redactText, typed[index])
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, inner := range typed {
			result[key] = redactValue(redactText, inner)
		}
		return result
	default:
		return value
	}
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

func currentHTTPTimeout(cfg config.Config) time.Duration {
	seconds := cfg.HTTP.TimeoutSeconds
	if seconds <= 0 {
		seconds = defaultHTTPTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}

func currentHTTPMaxRetries(cfg config.Config) int {
	if cfg.HTTP.MaxRetries < 0 {
		return defaultHTTPMaxRetries
	}
	if cfg.HTTP.MaxRetries == 0 {
		return 0
	}
	return cfg.HTTP.MaxRetries
}

func currentHTTPActionTimeout(action runtime.Action) time.Duration {
	if action.HTTPTimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(action.HTTPTimeoutSeconds) * time.Second
}

func cloneHTTPHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(headers))
	for key, value := range headers {
		cloned[key] = value
	}
	return cloned
}

func requiredActionString(data map[string]any, key string) (string, error) {
	if len(data) == 0 {
		return "", &runtime.Error{
			Code:    "plugin.protocol_violation",
			Message: fmt.Sprintf("onebot action missing %s", key),
		}
	}
	value, ok := data[key]
	if !ok {
		return "", &runtime.Error{
			Code:    "plugin.protocol_violation",
			Message: fmt.Sprintf("onebot action missing %s", key),
		}
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return "", &runtime.Error{
			Code:    "plugin.protocol_violation",
			Message: fmt.Sprintf("onebot action missing %s", key),
		}
	}
	return text, nil
}

func optionalActionString(data map[string]any, key string) (string, bool) {
	if len(data) == 0 {
		return "", false
	}
	value, ok := data[key]
	if !ok {
		return "", false
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return "", false
	}
	return text, true
}

func normalizeNumericValue(value any) any {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	default:
		return value
	}
}

func oneBotAPIValue(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	for _, char := range raw {
		if char < '0' || char > '9' {
			return raw
		}
	}
	return raw
}
