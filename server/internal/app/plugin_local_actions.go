package app

import (
	"context"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginconfig"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginfile"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginkv"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
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

type localActionService struct {
	state            *appRuntimeState
	grants           *pluginGrantView
	pluginConfig     pluginconfig.Repository
	pluginFiles      *pluginfile.Service
	pluginKV         pluginkv.Repository
	scheduler        *scheduler.Engine
	dispatcher       *dispatch.Dispatcher
	renderer         *render.Service
	adapter          *adapter.Shell
	webhookGateway   webhookGateway
	pluginLogLimiter *pluginLogLimiter
}

func newLocalActionService(deps localActionServiceDeps) *localActionService {
	return &localActionService{
		state:            deps.state,
		grants:           deps.grants,
		pluginConfig:     deps.pluginConfig,
		pluginFiles:      deps.pluginFiles,
		pluginKV:         deps.pluginKV,
		scheduler:        deps.scheduler,
		dispatcher:       deps.dispatcher,
		renderer:         deps.renderer,
		adapter:          deps.adapter,
		pluginLogLimiter: deps.pluginLogLimiter,
	}
}

func (s *localActionService) SetWebhookGateway(gateway webhookGateway) {
	if s == nil {
		return
	}
	s.webhookGateway = gateway
}

func (s *localActionService) Execute(ctx context.Context, pluginID, requestID string, action runtime.Action) (map[string]any, error) {
	switch action.Kind {
	case "logger.write":
		return s.executeLoggerWrite(ctx, pluginID, requestID, action)
	case "storage.kv":
		return s.executeStorageKV(ctx, pluginID, action)
	case "config.read":
		return s.executeConfigRead(ctx, pluginID, action)
	case "plugin.list":
		return s.executePluginList(ctx, pluginID)
	case "config.write":
		return s.executeConfigWrite(ctx, pluginID, action)
	case "storage.file":
		return s.executeStorageFile(ctx, pluginID, action)
	case "http.request":
		return s.executeHTTPRequest(ctx, pluginID, action)
	case "scheduler.create":
		return s.executeSchedulerCreate(ctx, pluginID, action)
	case "event.expose_webhook":
		return s.executeExposeWebhook(ctx, pluginID, action)
	case "render.image":
		return s.executeRenderImage(ctx, pluginID, action)
	default:
		switch {
		case runtimeIsOneBotLocalAction(action.Kind), runtimeIsProviderExtensionAction(action.Kind):
			return s.executeOneBotLocalAction(ctx, pluginID, requestID, action)
		default:
			return nil, &runtime.Error{
				Code:    "plugin.protocol_violation",
				Message: "received unsupported local action kind",
			}
		}
	}
}
