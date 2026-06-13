package app

import (
	"context"
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

type thirdPartyHTTPHandlers struct {
	accounts         thirdPartyAccountService
	accountValidator thirdPartyCredentialValidator
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

func newThirdPartyHTTPHandlers(accounts thirdPartyAccountService, accountValidator thirdPartyCredentialValidator, monitors thirdPartyMonitorService, transport http.RoundTripper) *thirdPartyHTTPHandlers {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &thirdPartyHTTPHandlers{
		accounts:         accounts,
		accountValidator: accountValidator,
		monitors:         monitors,
		mediaClient:      &http.Client{Transport: transport, Timeout: 20 * time.Second},
	}
}
