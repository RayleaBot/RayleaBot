package localaction

import (
	"context"
	"log/slog"

	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/adapter/shell"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginconfig"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginfile"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginkv"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

type Deps struct {
	CurrentConfig    func() config.Config
	Logger           *slog.Logger
	RedactText       func(string) string
	Grants           GrantView
	PluginConfig     pluginconfig.Repository
	PluginFiles      *pluginfile.Service
	PluginKV         pluginkv.Repository
	Secrets          secrets.Store
	Scheduler        *scheduler.Engine
	Dispatcher       *dispatch.Dispatcher
	Renderer         *renderservice.Service
	Adapter          *adaptershell.Shell
	PluginLogLimiter *PluginLogLimiter
	Governance       GovernanceService
	ThirdParty       ThirdPartyAccounts
	BilibiliSession  BilibiliSession
	RefreshCommands  func(context.Context, string, map[string]any)
}

type Service struct {
	currentConfig    func() config.Config
	logger           *slog.Logger
	redactText       func(string) string
	grants           GrantView
	pluginConfig     pluginconfig.Repository
	pluginFiles      *pluginfile.Service
	pluginKV         pluginkv.Repository
	secrets          secrets.Store
	scheduler        *scheduler.Engine
	dispatcher       *dispatch.Dispatcher
	renderer         *renderservice.Service
	adapter          *adaptershell.Shell
	webhookGateway   WebhookGateway
	pluginLogLimiter *PluginLogLimiter
	governance       GovernanceService
	thirdParty       ThirdPartyAccounts
	bilibiliSession  BilibiliSession
	refreshCommands  func(context.Context, string, map[string]any)
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
