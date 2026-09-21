package actions

import (
	"github.com/RayleaBot/RayleaBot/server/internal/bot/conversation"
	"log/slog"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/settings"
)

type Deps struct {
	CurrentConfig        func() config.Config
	Logger               *slog.Logger
	RedactText           func(string) string
	Plugins              PluginCatalog
	CallService          ServiceCallFunc
	Settings             *settings.Service
	PluginKV             KVRepository
	Conversations        *conversation.Registry
	Browser              BrowserSessionManager
	Scheduler            SchedulerCreateFunc
	MessageSender        MessageSendFunc
	Renderer             Renderer
	ResolveOneBotAdapter func(sourceAdapter, sourceProtocol string) (OneBotAdapter, error)
	Governance           GovernanceService
	// PluginDataRoot holds each plugin's data directory, <root>/<plugin_id>;
	// render.image reads path resources from the caller's directory.
	PluginDataRoot string
}

type Service struct{ actionRegistry *Registry }

func New(deps Deps) *Service {
	return &Service{actionRegistry: NewDefaultRegistry(deps)}
}
