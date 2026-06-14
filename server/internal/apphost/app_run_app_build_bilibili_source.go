package apphost

import (
	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/bilibili/session"
	bilibilisource "github.com/RayleaBot/RayleaBot/server/internal/bilibili/source"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

func buildBilibiliSourceService(
	platform appPlatform,
	pluginStack appPlugins,
	thirdPartyService *thirdparty.Service,
	bilibiliSession *bilibilisession.SessionClient,
	bilibiliEvents *managementevents.BilibiliSourceService,
	options Options,
) (*bilibilisource.Source, error) {
	return bilibilisource.NewSource(bilibilisource.Deps{
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
