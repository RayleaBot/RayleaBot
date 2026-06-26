package actions

import (
	"context"
	"log/slog"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

type Deps struct {
	CurrentConfig    func() config.Config
	Logger           *slog.Logger
	RedactText       func(string) string
	Capabilities     CapabilityView
	PluginConfig     PluginConfigRepository
	PluginFiles      FileStore
	PluginKV         KVRepository
	Secrets          SecretReader
	ThirdParty       ThirdPartyAccountReader
	Scheduler        SchedulerCreateFunc
	Dispatcher       ConfigChangeDispatcher
	Renderer         Renderer
	Adapter          OneBotAdapter
	PluginLogLimiter *PluginLogLimiter
	Governance       GovernanceService
	RefreshCommands  func(context.Context, string, map[string]any)
	WebhookGateway   func() WebhookGateway
	Registrars       []Registrar
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
		deps.RefreshCommands = func(ctx context.Context, pluginID string, settings map[string]any) {
			if hooks.refreshCommands != nil {
				hooks.refreshCommands(ctx, pluginID, settings)
			}
		}
		deps.WebhookGateway = func() WebhookGateway {
			if hooks == nil {
				return nil
			}
			return hooks.webhookGateway
		}
		service.actionRegistry = NewRegistryWithRegistrars(deps, deps.Registrars...)
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
