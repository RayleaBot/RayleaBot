package app

import (
	source "github.com/RayleaBot/RayleaBot/server/internal/bilibili"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

func buildBilibiliSourceService(
	platform appPlatform,
	pluginStack appPlugins,
	thirdPartyService *thirdparty.Service,
	bilibiliSession *source.SessionClient,
	bilibiliEvents *managementevents.BilibiliSourceService,
	options Options,
) (*source.Source, error) {
	return source.NewSource(source.Deps{
		Store:         platform.Storage,
		Accounts:      thirdPartyService,
		PluginConfig:  pluginStack.pluginConfig,
		Dispatcher:    pluginStack.Dispatcher,
		NotifyStatus:  bilibiliEvents.Publish,
		HTTPTransport: options.BilibiliHTTPTransport,
		Session:       bilibiliSession,
		Now:           options.BilibiliClock,
	})
}
