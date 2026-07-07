package actions

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

const defaultPluginLogRateLimit = "200/10s"

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

func logWriteRegistrar() registrar {
	return registrar{
		metadata: Metadata{
			Action:         "logger.write",
			Capability:     "logger.write",
			RequestSchema:  "plugin-protocol.action_logger_write",
			ResponseSchema: "plugin-protocol.local_action_result",
			AuditFields:    []string{"plugin_id", "request_id", "level"},
			ErrorCodes:     commonErrorCodes("platform.rate_limited"),
		},
		factory: func(deps Deps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return executeLogWrite(ctx, deps, req)
			}
		},
	}
}

func executeLogWrite(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.Capabilities == nil || !deps.Capabilities.CapabilityDeclared(ctx, req.PluginID, "logger.write") {
		return nil, &pluginruntime.Error{Code: "plugin.capability_violation", Message: "logger.write capability is not declared"}
	}
	if deps.PluginLogLimiter != nil && !deps.PluginLogLimiter.Allow(req.PluginID) {
		return nil, &pluginruntime.Error{Code: "platform.rate_limited", Message: "plugin log throughput exceeded the configured platform limit"}
	}
	if deps.Logger == nil {
		return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "logger.write is not available"}
	}

	level := strings.TrimSpace(req.Action.LogLevel)
	message := req.Action.LogMessage
	if deps.RedactText != nil {
		message = deps.RedactText(message)
	}
	attrs := []any{
		"component", "plugin",
		"plugin_id", req.PluginID,
		"request_id", req.RequestID,
	}
	keys := make([]string, 0, len(req.Action.LogFields))
	for key := range req.Action.LogFields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		attrs = append(attrs, key, redactLogValue(deps.RedactText, req.Action.LogFields[key]))
	}

	switch level {
	case "debug":
		deps.Logger.Debug(message, attrs...)
	case "warn":
		deps.Logger.Warn(message, attrs...)
	case "error":
		deps.Logger.Error(message, attrs...)
	default:
		deps.Logger.Info(message, attrs...)
	}
	return map[string]any{}, nil
}

func redactLogValue(redactText func(string) string, value any) any {
	switch typed := value.(type) {
	case string:
		if redactText == nil {
			return typed
		}
		return redactText(typed)
	case []any:
		result := make([]any, len(typed))
		for index := range typed {
			result[index] = redactLogValue(redactText, typed[index])
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, inner := range typed {
			result[key] = redactLogValue(redactText, inner)
		}
		return result
	default:
		return value
	}
}
