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
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/douyin"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/netease_music"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/weibo"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

type ThirdPartyService = thirdparty.Service
type ThirdPartyQRLoginService = common.Service
type BilibiliSource = bilibili.Source
type BilibiliSourceEvents = managementevents.BilibiliSourceService

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
	ThirdParty        *ThirdPartyService
	ThirdPartyQRLogin *ThirdPartyQRLoginService
	UserResolver      *UserResolver
	BilibiliSource    *BilibiliSource
	BilibiliEvents    *BilibiliSourceEvents
	AccountValidator  *AccountValidator
	HTTPCredentials   *bilibili.HTTPCredentialInjector
}

type DouyinUserResolver interface {
	ResolveUser(context.Context, string, []map[string]string) ([]thirdparty.AccountProfile, bool, error)
}

type UserResolver struct {
	client *http.Client
	douyin DouyinUserResolver
}

func NewUserResolver(transport http.RoundTripper, douyinResolver DouyinUserResolver) *UserResolver {
	return &UserResolver{
		client: common.NewHTTPClient(transport),
		douyin: douyinResolver,
	}
}

func (r *UserResolver) ResolveProfiles(ctx context.Context, platform string, query string, cookieMaps []map[string]string) ([]thirdparty.AccountProfile, bool, error) {
	if r == nil {
		return nil, false, nil
	}
	switch platform {
	case thirdparty.PlatformWeibo:
		return weibo.ResolveUserWithCookies(ctx, r.client, query, cookieMaps)
	case thirdparty.PlatformDouyin:
		return douyin.ResolveUserWithBrowser(ctx, r.client, query, cookieMaps, r.douyin)
	case thirdparty.PlatformNeteaseMusic:
		return netease_music.ResolveUser(ctx, r.client, query)
	default:
		return nil, false, nil
	}
}

type AccountValidator struct {
	bilibili *bilibili.AccountClient
	common   *common.AccountValidator
}

func NewAccountValidator(transport http.RoundTripper, now func() time.Time, bilibiliClient *bilibili.AccountClient) *AccountValidator {
	validator := common.NewAccountValidator(transport, now)
	validator.RegisterPlatform(thirdparty.PlatformWeibo, func(ctx context.Context, client *http.Client, cookies map[string]string) (thirdparty.AccountProfile, error) {
		return weibo.FetchAccountProfile(ctx, client, cookies)
	})
	validator.RegisterPlatform(thirdparty.PlatformDouyin, func(ctx context.Context, client *http.Client, cookies map[string]string) (thirdparty.AccountProfile, error) {
		return douyin.FetchAccountProfile(ctx, client, cookies)
	})
	validator.RegisterPlatform(thirdparty.PlatformNeteaseMusic, func(ctx context.Context, client *http.Client, cookies map[string]string) (thirdparty.AccountProfile, error) {
		return netease_music.FetchAccountProfile(ctx, client, cookies)
	})
	return &AccountValidator{
		bilibili: bilibiliClient,
		common:   validator,
	}
}

func (v *AccountValidator) CheckCookie(ctx context.Context, platform string, cookie string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
	normalized, err := thirdparty.NormalizePlatform(platform)
	if err != nil {
		return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{}, err
	}
	if normalized == thirdparty.PlatformBilibili {
		if v == nil || v.bilibili == nil {
			return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{}, thirdparty.ErrInvalidAccount
		}
		return v.bilibili.CheckCookie(ctx, cookie)
	}
	if v == nil || v.common == nil {
		return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{}, thirdparty.ErrInvalidAccount
	}
	return v.common.CheckCookie(ctx, normalized, cookie)
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
		bilibili.Platform:      bilibili.NewLoginProvider(deps.HTTPTransport, deps.Clock),
		weibo.Platform:         weibo.NewProvider(common.NewHTTPClient(deps.HTTPTransport)),
		douyin.Platform:        douyin.NewProvider(common.NewHTTPClient(deps.HTTPTransport), douyinBrowser),
		netease_music.Platform: netease_music.NewProvider(common.NewHTTPClient(deps.HTTPTransport)),
	}, deps.Clock, common.WithAccountStore(thirdPartyService))
	bilibiliEvents := managementevents.NewBilibiliSourceService()
	bilibiliModule, err := bilibili.Build(bilibili.Deps{
		Store: bilibili.Store{
			Read:  deps.Platform.Storage.Read,
			Write: deps.Platform.Storage.Write,
		},
		Accounts:      thirdPartyService,
		PluginConfig:  deps.Plugins.PluginConfig,
		Dispatcher:    bilibiliEventDispatcher{dispatcher: deps.Events.Dispatcher},
		NotifyStatus:  bilibiliEvents.Publish,
		HTTPTransport: deps.HTTPTransport,
		Now:           deps.Clock,
	})
	if err != nil {
		return State{}, err
	}

	return State{
		ThirdParty:        thirdPartyService,
		ThirdPartyQRLogin: thirdPartyQRLogin,
		UserResolver:      NewUserResolver(deps.HTTPTransport, douyinBrowser),
		BilibiliSource:    bilibiliModule.Source,
		BilibiliEvents:    bilibiliEvents,
		AccountValidator:  NewAccountValidator(deps.HTTPTransport, deps.Clock, bilibiliModule.AccountClient),
		HTTPCredentials:   bilibiliModule.HTTPCredentials,
	}, nil
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
