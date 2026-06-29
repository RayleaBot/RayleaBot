package integrationmodule

import (
	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/douyin"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/netease_music"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/qrcode"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/weibo"
)

func buildQRLoginService(deps Deps, accountStore *thirdparty.Service) *qrcode.Service {
	browserPath, browserArgs := browserLaunchConfig(deps)
	douyinBrowser := douyin.NewChromedpBrowser(browserPath, browserArgs, deps.HTTPTransport)
	return qrcode.NewService(map[string]qrcode.Provider{
		bilibilisession.Platform: bilibilisession.NewProvider(deps.HTTPTransport, deps.Clock),
		weibo.Platform:           weibo.NewProvider(thirdparty.NewHTTPClient(deps.HTTPTransport)),
		douyin.Platform:          douyin.NewProvider(thirdparty.NewHTTPClient(deps.HTTPTransport), douyinBrowser),
		netease_music.Platform:   netease_music.NewProvider(thirdparty.NewHTTPClient(deps.HTTPTransport)),
	}, deps.Clock, qrcode.WithAccountStore(accountStore))
}

func browserLaunchConfig(deps Deps) (string, []string) {
	browserPath := deps.Config.Render.BrowserPath
	browserArgs := deps.Config.Render.BrowserArgs
	if deps.Renderer != nil {
		browserPath, browserArgs = deps.Renderer.BrowserLaunchConfig()
	}
	return browserPath, browserArgs
}
