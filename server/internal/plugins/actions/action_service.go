package actions

import (
	"log/slog"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/settings"
)

type Deps struct {
	CurrentConfig        func() config.Config
	Logger               *slog.Logger
	RedactText           func(string) string
	Permissions          PermissionView
	Plugins              plugins.CatalogView
	Settings             *settings.Service
	PluginFiles          FileStore
	PluginKV             KVRepository
	ThirdParty           ThirdPartyAccountReader
	AccountValidation    ThirdPartyAccountValidationRequester
	ThirdPartyResolve    ThirdPartyResolver
	Scheduler            SchedulerCreateFunc
	MessageSender        MessageSendFunc
	Renderer             Renderer
	ResolveOneBotAdapter func(sourceAdapter, sourceProtocol string) (OneBotAdapter, error)
	PluginLogLimiter     *PluginLogLimiter
	Governance           GovernanceService
}

type Service struct{ actionRegistry *Registry }

func New(deps Deps) *Service {
	return &Service{actionRegistry: NewDefaultRegistry(deps)}
}
