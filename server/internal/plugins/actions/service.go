package actions

import (
	"context"
	"log/slog"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/storageaction"
)

type Deps struct {
	CurrentConfig    func() config.Config
	Logger           *slog.Logger
	RedactText       func(string) string
	Capabilities     CapabilityView
	PluginConfig     PluginConfigRepository
	PluginFiles      storageaction.FileStore
	PluginKV         storageaction.KVRepository
	Secrets          SecretReader
	Scheduler        SchedulerCreateFunc
	Dispatcher       ConfigChangeDispatcher
	Renderer         Renderer
	Adapter          OneBotAdapter
	PluginLogLimiter *PluginLogLimiter
	Governance       GovernanceService
	HTTPCredentials  HTTPCredentialInjector
	RefreshCommands  func(context.Context, string, map[string]any)
	ActionRegistry   *Registry
}

type Service struct {
	actionRegistry *Registry
	runtimeHooks   *runtimeHooks
}

type runtimeHooks struct {
	refreshCommands func(context.Context, string, map[string]any)
	webhookGateway  WebhookGateway
}

func New(deps Deps) *Service {
	hooks := &runtimeHooks{
		refreshCommands: deps.RefreshCommands,
	}
	service := &Service{
		runtimeHooks: hooks,
	}
	if deps.ActionRegistry != nil {
		service.actionRegistry = deps.ActionRegistry
	} else {
		service.actionRegistry = defaultRegistry(registryDeps{
			currentConfig:    deps.CurrentConfig,
			logger:           deps.Logger,
			redactText:       deps.RedactText,
			capabilities:     deps.Capabilities,
			pluginConfig:     deps.PluginConfig,
			pluginFiles:      deps.PluginFiles,
			pluginKV:         deps.PluginKV,
			secrets:          deps.Secrets,
			scheduler:        deps.Scheduler,
			dispatcher:       deps.Dispatcher,
			renderer:         deps.Renderer,
			adapter:          deps.Adapter,
			pluginLogLimiter: deps.PluginLogLimiter,
			governance:       deps.Governance,
			httpCredentials:  deps.HTTPCredentials,
			runtimeHooks:     hooks,
		})
	}
	return service
}

func (s *Service) SetRefreshPluginCommands(refresh func(context.Context, string, map[string]any)) {
	if s == nil {
		return
	}
	if s.runtimeHooks == nil {
		s.runtimeHooks = &runtimeHooks{}
	}
	s.runtimeHooks.refreshCommands = refresh
}

func (s *Service) SetWebhookGateway(gateway WebhookGateway) {
	if s == nil {
		return
	}
	if s.runtimeHooks == nil {
		s.runtimeHooks = &runtimeHooks{}
	}
	s.runtimeHooks.webhookGateway = gateway
}
