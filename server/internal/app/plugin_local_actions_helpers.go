package app

import (
	"context"
	"strings"
	"time"

	"rayleabot/server/internal/config"
	"rayleabot/server/internal/permission"
	"rayleabot/server/internal/pluginfile"
	"rayleabot/server/internal/pluginkv"
)

func (a *App) pluginCapabilityGranted(ctx context.Context, pluginID, capability string) bool {
	if a == nil || a.pluginLifecycle == nil {
		return false
	}
	for _, granted := range a.pluginLifecycle.grantedCapabilities(ctx, pluginID) {
		if strings.TrimSpace(granted) == capability {
			return true
		}
	}
	return false
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

func (a *App) redactString(value string) string {
	if a == nil || a.redactText == nil {
		return value
	}
	return a.redactText(value)
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

func newPluginLogLimiter(cfg config.Config) *pluginLogLimiter {
	return &pluginLogLimiter{
		now:     time.Now,
		limit:   parsePluginLogRateLimit(cfg),
		records: make(map[string][]time.Time),
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
	limit, err := permission.ParseRateLimit(strings.TrimSpace(cfg.Logging.RateLimitPerPlugin))
	if err == nil {
		return limit
	}
	limit, _ = permission.ParseRateLimit(defaultPluginLogRateLimit)
	return limit
}
