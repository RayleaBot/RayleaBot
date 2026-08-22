package app

import (
	"log/slog"
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
	Config               config.Config
	Platform             PlatformState
	Renderer             integrationRenderer
	HTTPTransport        http.RoundTripper
	Clock                func() time.Time
	Logger               *slog.Logger
	NotifyAccountChanged func()
}

type integrationState struct {
	ThirdParty        *thirdparty.Service
	ThirdPartyQRLogin *thirdparty.QRLoginService
	AccountValidator  *accountvalidation.Validator
	AccountValidation *accountvalidation.Service
}

func buildIntegrations(deps integrationDeps) (integrationState, error) {
	thirdPartyService, err := thirdparty.NewService(deps.Platform.Storage, deps.Platform.Secrets)
	if err != nil {
		return integrationState{}, err
	}

	validator := accountvalidation.NewDefault(deps.HTTPTransport, deps.Clock)
	validationService, err := accountvalidation.NewService(
		thirdPartyService,
		validator,
		deps.Config.ThirdParty.CredentialCheckIntervalMinutes,
		deps.Logger,
		deps.Clock,
		deps.NotifyAccountChanged,
	)
	if err != nil {
		return integrationState{}, err
	}

	return integrationState{
		ThirdParty:        thirdPartyService,
		ThirdPartyQRLogin: buildQRLoginService(deps, thirdPartyService),
		AccountValidator:  validator,
		AccountValidation: validationService,
	}, nil
}

func buildQRLoginService(deps integrationDeps, accountStore *thirdparty.Service) *thirdparty.QRLoginService {
	configuredBrowserPath, managedBrowserPath, browserArgs := browserLaunchConfig(deps)
	douyinBrowser := douyin.NewChromedpBrowser(douyin.BrowserOptions{
		ConfiguredBrowserPath: configuredBrowserPath,
		ManagedBrowserPath:    managedBrowserPath,
		BrowserArgs:           browserArgs,
		Mode:                  deps.Config.ThirdParty.DouyinLogin.BrowserMode,
		RemoteDebuggingURL:    deps.Config.ThirdParty.DouyinLogin.RemoteDebuggingURL,
		Logger:                deps.Logger,
	})
	return thirdparty.NewQRLoginService(map[string]thirdparty.QRLoginProvider{
		bilibilisession.Platform: bilibilisession.NewProvider(deps.HTTPTransport, deps.Clock),
		weibo.Platform:           weibo.NewProvider(thirdparty.NewHTTPClient(deps.HTTPTransport)),
		douyin.Platform:          douyin.NewProvider(thirdparty.NewHTTPClient(deps.HTTPTransport), douyinBrowser),
		netease_music.Platform:   netease_music.NewProvider(thirdparty.NewHTTPClient(deps.HTTPTransport)),
	}, deps.Clock, thirdparty.WithQRLoginAccountStore(accountStore))
}

func browserLaunchConfig(deps integrationDeps) (string, string, []string) {
	configuredPath := deps.Config.Render.BrowserPath
	managedPath := configuredPath
	browserArgs := deps.Config.Render.BrowserArgs
	if deps.Renderer != nil {
		managedPath, browserArgs = deps.Renderer.BrowserLaunchConfig()
	}
	return configuredPath, managedPath, browserArgs
}
