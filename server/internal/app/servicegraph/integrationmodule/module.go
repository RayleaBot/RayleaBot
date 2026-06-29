package integrationmodule

import (
	"net/http"
	"time"

	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/qrcode"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
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

func Build(deps Deps) (State, error) {
	thirdPartyService, err := thirdparty.NewService(deps.Platform.Storage, deps.Platform.Secrets)
	if err != nil {
		return State{}, err
	}

	return State{
		ThirdParty:        thirdPartyService,
		ThirdPartyQRLogin: buildQRLoginService(deps, thirdPartyService),
		AccountValidator:  newDefaultAccountValidator(deps.HTTPTransport, deps.Clock),
	}, nil
}
