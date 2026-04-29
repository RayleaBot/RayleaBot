package localaction

import (
	"context"
	"log/slog"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginconfig"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginfile"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginkv"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
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

type GrantView interface {
	CapabilityGranted(context.Context, string, string) bool
	StorageRootGranted(context.Context, string, string) bool
	GrantedHTTPHosts(context.Context, string) []string
	GrantedWebhookScope(context.Context, string, string) (plugins.WebhookScope, bool)
	ListPluginSnapshots() []plugins.Snapshot
}

type WebhookGateway interface {
	Expose(context.Context, string, runtime.Action) (map[string]any, error)
}

type GovernanceService interface {
	ReadBlacklist(context.Context) (governance.BlacklistSnapshot, error)
	UpsertBlacklistEntry(context.Context, string, string, string) (governance.EntryResponse, error)
	DeleteBlacklistEntry(context.Context, string, string) error
	ReadWhitelist(context.Context) (governance.WhitelistSnapshot, error)
	SetWhitelistEnabled(context.Context, bool) (governance.WhitelistStateResponse, error)
	UpsertWhitelistEntry(context.Context, string, string, string) (governance.EntryResponse, error)
	DeleteWhitelistEntry(context.Context, string, string) error
	ReadCommandPolicy(context.Context) (governance.CommandPolicyResponse, error)
}

type Deps struct {
	CurrentConfig    func() config.Config
	Logger           *slog.Logger
	RedactText       func(string) string
	Grants           GrantView
	PluginConfig     pluginconfig.Repository
	PluginFiles      *pluginfile.Service
	PluginKV         pluginkv.Repository
	Scheduler        *scheduler.Engine
	Dispatcher       *dispatch.Dispatcher
	Renderer         *render.Service
	Adapter          *adapter.Shell
	PluginLogLimiter *PluginLogLimiter
	Governance       GovernanceService
}

type Service struct {
	currentConfig    func() config.Config
	logger           *slog.Logger
	redactText       func(string) string
	grants           GrantView
	pluginConfig     pluginconfig.Repository
	pluginFiles      *pluginfile.Service
	pluginKV         pluginkv.Repository
	scheduler        *scheduler.Engine
	dispatcher       *dispatch.Dispatcher
	renderer         *render.Service
	adapter          *adapter.Shell
	webhookGateway   WebhookGateway
	pluginLogLimiter *PluginLogLimiter
	governance       GovernanceService
}

func New(deps Deps) *Service {
	return &Service{
		currentConfig:    deps.CurrentConfig,
		logger:           deps.Logger,
		redactText:       deps.RedactText,
		grants:           deps.Grants,
		pluginConfig:     deps.PluginConfig,
		pluginFiles:      deps.PluginFiles,
		pluginKV:         deps.PluginKV,
		scheduler:        deps.Scheduler,
		dispatcher:       deps.Dispatcher,
		renderer:         deps.Renderer,
		adapter:          deps.Adapter,
		pluginLogLimiter: deps.PluginLogLimiter,
		governance:       deps.Governance,
	}
}

func (s *Service) SetWebhookGateway(gateway WebhookGateway) {
	if s == nil {
		return
	}
	s.webhookGateway = gateway
}

func (s *Service) Execute(ctx context.Context, pluginID, requestID string, action runtime.Action, parentEvent runtime.Event) (map[string]any, error) {
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
	case "governance.blacklist.read":
		return s.executeGovernanceBlacklistRead(ctx, pluginID)
	case "governance.blacklist.write":
		return s.executeGovernanceBlacklistWrite(ctx, pluginID, action)
	case "governance.whitelist.read":
		return s.executeGovernanceWhitelistRead(ctx, pluginID)
	case "governance.whitelist.write":
		return s.executeGovernanceWhitelistWrite(ctx, pluginID, action)
	case "governance.command_policy.read":
		return s.executeGovernanceCommandPolicyRead(ctx, pluginID)
	case "storage.file":
		return s.executeStorageFile(ctx, pluginID, action)
	case "http.request":
		return s.executeHTTPRequest(ctx, pluginID, action)
	case "scheduler.create":
		return s.executeSchedulerCreate(ctx, pluginID, action)
	case "event.expose_webhook":
		return s.executeExposeWebhook(ctx, pluginID, action)
	case "render.image":
		return s.executeRenderImage(ctx, pluginID, action, parentEvent)
	default:
		switch {
		case runtimeIsOneBotLocalAction(action.Kind), runtimeIsProviderExtensionAction(action.Kind):
			return s.executeOneBotLocalAction(ctx, pluginID, action)
		default:
			return nil, &runtime.Error{
				Code:    "plugin.protocol_violation",
				Message: "received unsupported local action kind",
			}
		}
	}
}

func (s *Service) config() config.Config {
	if s == nil || s.currentConfig == nil {
		return config.Config{}
	}
	return s.currentConfig()
}
