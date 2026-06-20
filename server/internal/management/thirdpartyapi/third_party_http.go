package thirdpartyapi

import (
	"context"
	"net/http"
	"time"

	thirdpartylogin "github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdpartylogin"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

type ThirdPartyHandlers struct {
	accounts         thirdPartyAccountService
	accountValidator thirdPartyCredentialValidator
	qrLogin          thirdPartyQRCodeLoginService
	monitors         thirdPartyMonitorService
	mediaClient      *http.Client
}

type thirdPartyAccountService interface {
	List(context.Context) ([]thirdparty.Account, error)
	Upsert(context.Context, thirdparty.UpsertRequest) (thirdparty.Account, error)
	Delete(context.Context, string, string) error
}

type thirdPartyCredentialValidator interface {
	CheckCookie(context.Context, string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error)
}

type thirdPartyQRCodeLoginService interface {
	Create(context.Context, string) (thirdpartylogin.CreateResult, error)
	Poll(context.Context, string, string) (thirdpartylogin.PollResult, error)
}

func NewThirdPartyHandlers(accounts thirdPartyAccountService, accountValidator thirdPartyCredentialValidator, qrLogin thirdPartyQRCodeLoginService, monitors thirdPartyMonitorService, transport http.RoundTripper) *ThirdPartyHandlers {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &ThirdPartyHandlers{
		accounts:         accounts,
		accountValidator: accountValidator,
		qrLogin:          qrLogin,
		monitors:         monitors,
		mediaClient:      &http.Client{Transport: transport, Timeout: 20 * time.Second},
	}
}
