package servicegraph

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/app/eventstack"
	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/app/pluginstack"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	bilibilisource "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/source"
	bilibilisubscriptions "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/subscriptions"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

func buildBilibiliSourceService(
	platform appplatform.State,
	pluginStack pluginstack.State,
	eventStack eventstack.State,
	thirdPartyService *thirdparty.Service,
	bilibiliSession *bilibilisession.SessionClient,
	bilibiliEvents *managementevents.BilibiliSourceService,
	deps BuildDeps,
) (*bilibilisource.Source, error) {
	return bilibilisource.NewSource(bilibilisource.Deps{
		Store:         bilibilisource.Store{Read: platform.Storage.Read, Write: platform.Storage.Write},
		Accounts:      thirdPartyService,
		Subjects:      bilibilisubscriptions.NewPluginConfigProvider(pluginStack.PluginConfig),
		Dispatcher:    bilibiliEventDispatcher{dispatcher: eventStack.Dispatcher},
		NotifyStatus:  bilibiliEvents.Publish,
		HTTPTransport: deps.BilibiliHTTPTransport,
		Session:       bilibiliSession,
		Now:           deps.BilibiliClock,
	})
}

type bilibiliEventDispatcher struct {
	dispatcher *dispatch.Dispatcher
}

func (d bilibiliEventDispatcher) Dispatch(ctx context.Context, event runtimeprotocol.Event, commandName string) {
	if d.dispatcher == nil {
		return
	}
	d.dispatcher.Dispatch(ctx, event, commandName)
}
