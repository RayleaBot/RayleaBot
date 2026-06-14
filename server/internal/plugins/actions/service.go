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
	Grants           GrantView
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
	ThirdParty       ThirdPartyAccounts
	BilibiliSession  BilibiliSession
	RefreshCommands  func(context.Context, string, map[string]any)
	ActionRegistry   *Registry
}

type Service struct {
	currentConfig    func() config.Config
	logger           *slog.Logger
	redactText       func(string) string
	grants           GrantView
	pluginConfig     PluginConfigRepository
	pluginFiles      storageaction.FileStore
	pluginKV         storageaction.KVRepository
	secrets          SecretReader
	scheduler        SchedulerCreateFunc
	dispatcher       ConfigChangeDispatcher
	renderer         Renderer
	adapter          OneBotAdapter
	webhookGateway   WebhookGateway
	pluginLogLimiter *PluginLogLimiter
	governance       GovernanceService
	thirdParty       ThirdPartyAccounts
	bilibiliSession  BilibiliSession
	refreshCommands  func(context.Context, string, map[string]any)
	actionRegistry   *Registry
}

func New(deps Deps) *Service {
	registry := deps.ActionRegistry
	if registry == nil {
		registry = DefaultRegistry()
	}
	return &Service{
		currentConfig:    deps.CurrentConfig,
		logger:           deps.Logger,
		redactText:       deps.RedactText,
		grants:           deps.Grants,
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
		thirdParty:       deps.ThirdParty,
		bilibiliSession:  deps.BilibiliSession,
		refreshCommands:  deps.RefreshCommands,
		actionRegistry:   registry,
	}
}

func (s *Service) SetRefreshPluginCommands(refresh func(context.Context, string, map[string]any)) {
	if s == nil {
		return
	}
	s.refreshCommands = refresh
}

func (s *Service) SetWebhookGateway(gateway WebhookGateway) {
	if s == nil {
		return
	}
	s.webhookGateway = gateway
}

func (s *Service) config() config.Config {
	if s == nil || s.currentConfig == nil {
		return config.Config{}
	}
	return s.currentConfig()
}
