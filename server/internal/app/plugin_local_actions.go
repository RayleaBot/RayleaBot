package app

import (
	"context"
	"sync"
	"time"

	"rayleabot/server/internal/permission"
	"rayleabot/server/internal/runtime"
)

const (
	defaultPluginLogRateLimit   = "200/10s"
	defaultKVValueMaxBytes      = 65536
	defaultKVTotalLimitMegabyte = 16
	defaultFileMaxBytes         = 10 * 1024 * 1024
	defaultPluginWorkdirMB      = 256
	defaultHTTPTimeoutSeconds   = 10
	defaultHTTPMaxRetries       = 2
)

type pluginLogLimiter struct {
	mu      sync.Mutex
	now     func() time.Time
	limit   permission.RateLimit
	records map[string][]time.Time
}

func (l *pluginLogLimiter) SetLimit(limit permission.RateLimit) {
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

func (l *pluginLogLimiter) Allow(pluginID string) bool {
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

func (a *App) executeLocalAction(ctx context.Context, pluginID, requestID string, action runtime.Action) (map[string]any, error) {
	switch action.Kind {
	case "logger.write":
		return a.executeLoggerWrite(ctx, pluginID, requestID, action)
	case "storage.kv":
		return a.executeStorageKV(ctx, pluginID, action)
	case "config.read":
		return a.executeConfigRead(ctx, pluginID, action)
	case "config.write":
		return a.executeConfigWrite(ctx, pluginID, action)
	case "storage.file":
		return a.executeStorageFile(ctx, pluginID, action)
	case "http.request":
		return a.executeHTTPRequest(ctx, pluginID, action)
	case "scheduler.create":
		return a.executeSchedulerCreate(ctx, pluginID, action)
	case "event.expose_webhook":
		return a.executeExposeWebhook(ctx, pluginID, action)
	case "render.image":
		return a.executeRenderImage(ctx, pluginID, action)
	default:
		return nil, &runtime.Error{
			Code:    "plugin.protocol_violation",
			Message: "received unsupported local action kind",
		}
	}
}
