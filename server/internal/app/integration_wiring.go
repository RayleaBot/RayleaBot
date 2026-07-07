package app

import (
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/accountvalidation"
	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/douyin"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/netease_music"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/weibo"
)

type integrationRenderer interface {
	BrowserLaunchConfig() (string, []string)
}

type integrationDeps struct {
	Config        config.Config
	Platform      PlatformState
	Renderer      integrationRenderer
	HTTPTransport http.RoundTripper
	Clock         func() time.Time
}

type integrationState struct {
	ThirdParty        *thirdparty.Service
	ThirdPartyQRLogin *thirdparty.QRLoginService
	AccountValidator  *accountvalidation.Validator
}

func buildIntegrations(deps integrationDeps) (integrationState, error) {
	thirdPartyService, err := thirdparty.NewService(deps.Platform.Storage, deps.Platform.Secrets)
	if err != nil {
		return integrationState{}, err
	}

	return integrationState{
		ThirdParty:        thirdPartyService,
		ThirdPartyQRLogin: buildQRLoginService(deps, thirdPartyService),
		AccountValidator:  accountvalidation.NewDefault(deps.HTTPTransport, deps.Clock),
	}, nil
}

func buildQRLoginService(deps integrationDeps, accountStore *thirdparty.Service) *thirdparty.QRLoginService {
	browserPath, browserArgs := browserLaunchConfig(deps)
	douyinBrowser := douyin.NewChromedpBrowser(browserPath, browserArgs, deps.HTTPTransport)
	return thirdparty.NewQRLoginService(map[string]thirdparty.QRLoginProvider{
		bilibilisession.Platform: bilibilisession.NewProvider(deps.HTTPTransport, deps.Clock),
		weibo.Platform:           weibo.NewProvider(thirdparty.NewHTTPClient(deps.HTTPTransport)),
		douyin.Platform:          douyin.NewProvider(thirdparty.NewHTTPClient(deps.HTTPTransport), douyinBrowser),
		netease_music.Platform:   netease_music.NewProvider(thirdparty.NewHTTPClient(deps.HTTPTransport)),
	}, deps.Clock, thirdparty.WithQRLoginAccountStore(accountStore))
}

func browserLaunchConfig(deps integrationDeps) (string, []string) {
	browserPath := deps.Config.Render.BrowserPath
	browserArgs := deps.Config.Render.BrowserArgs
	if deps.Renderer != nil {
		browserPath, browserArgs = deps.Renderer.BrowserLaunchConfig()
	}
	return browserPath, browserArgs
}
