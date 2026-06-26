package integrationmodule

import (
	"context"
	"net/http"
	"time"

	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/douyin"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/netease_music"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/qrcode"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/weibo"
)

type ThirdPartyService = thirdparty.Service
type ThirdPartyQRLoginService = qrcode.Service

type Module = State

type Renderer interface {
	BrowserLaunchConfig() (string, []string)
}

type Deps struct {
	Config        config.Config
	Platform      appplatform.State
	Renderer      Renderer
	HTTPTransport http.RoundTripper
	Clock         func() time.Time
}

type State struct {
	ThirdParty        *ThirdPartyService
	ThirdPartyQRLogin *ThirdPartyQRLoginService
	AccountValidator  *AccountValidator
}

type AccountValidator struct {
	bilibili   *bilibilisession.AccountClient
	thirdParty *thirdparty.AccountValidator
}

func NewAccountValidator(transport http.RoundTripper, now func() time.Time, bilibiliClient *bilibilisession.AccountClient) *AccountValidator {
	validator := thirdparty.NewAccountValidator(transport, now)
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
		bilibili:   bilibiliClient,
		thirdParty: validator,
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
	if v == nil || v.thirdParty == nil {
		return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{}, thirdparty.ErrInvalidAccount
	}
	return v.thirdParty.CheckCookie(ctx, normalized, cookie)
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
	thirdPartyQRLogin := qrcode.NewService(map[string]qrcode.Provider{
		bilibilisession.Platform: bilibilisession.NewProvider(deps.HTTPTransport, deps.Clock),
		weibo.Platform:           weibo.NewProvider(thirdparty.NewHTTPClient(deps.HTTPTransport)),
		douyin.Platform:          douyin.NewProvider(thirdparty.NewHTTPClient(deps.HTTPTransport), douyinBrowser),
		netease_music.Platform:   netease_music.NewProvider(thirdparty.NewHTTPClient(deps.HTTPTransport)),
	}, deps.Clock, qrcode.WithAccountStore(thirdPartyService))

	return State{
		ThirdParty:        thirdPartyService,
		ThirdPartyQRLogin: thirdPartyQRLogin,
		AccountValidator:  NewAccountValidator(deps.HTTPTransport, deps.Clock, bilibilisession.NewAccountClient(deps.HTTPTransport, deps.Clock, nil)),
	}, nil
}
