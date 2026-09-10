package actions

import (
	"context"
	"log/slog"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type Deps struct {
	CurrentConfig        func() config.Config
	Logger               *slog.Logger
	RedactText           func(string) string
	Permissions          PermissionView
	Plugins              plugins.CatalogView
	PluginConfig         PluginConfigRepository
	PluginFiles          FileStore
	PluginKV             KVRepository
	Secrets              SecretReader
	ThirdParty           ThirdPartyAccountReader
	AccountValidation    ThirdPartyAccountValidationRequester
	ThirdPartyResolve    ThirdPartyResolver
	Scheduler            SchedulerCreateFunc
	Dispatcher           ConfigChangeDispatcher
	MessageSender        MessageSendFunc
	Renderer             Renderer
	ResolveOneBotAdapter func(sourceAdapter, sourceProtocol string) (OneBotAdapter, error)
	PluginLogLimiter     *PluginLogLimiter
	Governance           GovernanceService
	RefreshCommands      func(context.Context, string, map[string]any)
}

type Service struct{ actionRegistry *Registry }

func New(deps Deps) *Service {
	return &Service{actionRegistry: NewDefaultRegistry(deps)}
}
