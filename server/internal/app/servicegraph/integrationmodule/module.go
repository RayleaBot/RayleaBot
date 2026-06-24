package integrationmodule

import (
	"context"
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/app/eventstack"
	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/app/pluginstack"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	bilibilicredential "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/credential"
	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	bilibilisource "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/source"
	bilibilisubscriptions "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/subscriptions"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/douyin"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/netease_music"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/weibo"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

type ThirdPartyService = thirdparty.Service
type ThirdPartyQRLoginService = common.Service
type DouyinBrowser = douyin.ChromedpBrowser
type BilibiliSource = bilibilisource.Source
type BilibiliSourceEvents = managementevents.BilibiliSourceService
type BilibiliAccountClient = bilibilisession.AccountClient
type BilibiliQRLoginService = bilibilisession.QRLoginService

type Module = State

type Renderer interface {
	BrowserLaunchConfig() (string, []string)
}

type Deps struct {
	Config        config.Config
	Platform      appplatform.State
	Plugins       pluginstack.State
	Events        eventstack.State
	Renderer      Renderer
	HTTPTransport http.RoundTripper
	Clock         func() time.Time
}

type State struct {
	ThirdParty            *ThirdPartyService
	ThirdPartyQRLogin     *ThirdPartyQRLoginService
	DouyinBrowser         *DouyinBrowser
	BilibiliSource        *BilibiliSource
	BilibiliEvents        *BilibiliSourceEvents
	BilibiliAccountClient *BilibiliAccountClient
	BilibiliQRLogin       *BilibiliQRLoginService
	HTTPCredentials       localaction.HTTPCredentialInjector
}

func Build(deps Deps) (State, error) {
	thirdPartyService, err := thirdparty.NewService(deps.Platform.Storage, deps.Platform.Secrets)
	if err != nil {
		return State{}, err
	}

	browserPath := deps.Config.Render.BrowserPath
	browserArgs := deps.Config.Render.BrowserArgs
	if deps.Renderer != nil {
		browserPath, browserArgs = deps.Renderer.BrowserLaunchConfig()
	}
	douyinBrowser := douyin.NewChromedpBrowser(browserPath, browserArgs, deps.HTTPTransport)
	thirdPartyQRLogin := common.NewService(map[string]common.Provider{
		thirdparty.PlatformWeibo:        weibo.NewProvider(common.NewHTTPClient(deps.HTTPTransport)),
		thirdparty.PlatformDouyin:       douyin.NewProvider(common.NewHTTPClient(deps.HTTPTransport), douyinBrowser),
		thirdparty.PlatformNeteaseMusic: netease_music.NewProvider(common.NewHTTPClient(deps.HTTPTransport)),
	}, deps.Clock)
	bilibiliSession := bilibilisession.NewSessionClient(deps.HTTPTransport, deps.Clock, nil)
	bilibiliEvents := managementevents.NewBilibiliSourceService()
	bilibiliSource, err := buildBilibiliSourceService(deps, thirdPartyService, bilibiliSession, bilibiliEvents)
	if err != nil {
		return State{}, err
	}

	return State{
		ThirdParty:            thirdPartyService,
		ThirdPartyQRLogin:     thirdPartyQRLogin,
		DouyinBrowser:         douyinBrowser,
		BilibiliSource:        bilibiliSource,
		BilibiliEvents:        bilibiliEvents,
		BilibiliAccountClient: bilibilisession.NewAccountClient(deps.HTTPTransport, deps.Clock, nil),
		BilibiliQRLogin:       bilibilisession.NewQRLoginService(deps.HTTPTransport, deps.Clock),
		HTTPCredentials:       bilibilicredential.NewInjector(thirdPartyService, bilibiliSession),
	}, nil
}

func buildBilibiliSourceService(
	deps Deps,
	thirdPartyService *thirdparty.Service,
	bilibiliSession *bilibilisession.SessionClient,
	bilibiliEvents *managementevents.BilibiliSourceService,
) (*bilibilisource.Source, error) {
	return bilibilisource.NewSource(bilibilisource.Deps{
		Store:         bilibilisource.Store{Read: deps.Platform.Storage.Read, Write: deps.Platform.Storage.Write},
		Accounts:      thirdPartyService,
		Subjects:      bilibilisubscriptions.NewPluginConfigProvider(deps.Plugins.PluginConfig),
		Dispatcher:    bilibiliEventDispatcher{dispatcher: deps.Events.Dispatcher},
		NotifyStatus:  bilibiliEvents.Publish,
		HTTPTransport: deps.HTTPTransport,
		Session:       bilibiliSession,
		Now:           deps.Clock,
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
