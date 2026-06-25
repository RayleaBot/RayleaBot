package thirdpartyapi

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/qrcode"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"net/http"
	"time"
)

type ThirdPartyHandlers struct {
	accounts         thirdPartyAccountService
	accountValidator thirdPartyCredentialValidator
	qrLogin          thirdPartyQRCodeLoginService
	monitors         thirdPartyMonitorService
	userResolver     thirdPartyUserResolver
	mediaClient      *http.Client
}

type ModuleDeps struct {
	Accounts         thirdPartyAccountService
	AccountValidator thirdPartyCredentialValidator
	QRLogin          thirdPartyQRCodeLoginService
	Monitors         thirdPartyMonitorService
	Transport        http.RoundTripper
	UserResolver     thirdPartyUserResolver
}

type thirdPartyAccountService interface {
	List(context.Context) ([]thirdparty.Account, error)
	Upsert(context.Context, thirdparty.UpsertRequest) (thirdparty.Account, error)
	Delete(context.Context, string, string) error
}

type thirdPartyCredentialValidator interface {
	CheckCookie(context.Context, string, string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error)
}

type thirdPartyQRCodeLoginService interface {
	Create(context.Context, string) (qrcode.CreateResult, error)
	Poll(context.Context, string, string) (qrcode.PollResult, error)
}

type thirdPartyUserResolver interface {
	ResolveProfiles(context.Context, string, string, []map[string]string) ([]thirdparty.AccountProfile, bool, error)
}

type ThirdPartyHandlersOption func(*ThirdPartyHandlers)

func WithUserResolver(resolver thirdPartyUserResolver) ThirdPartyHandlersOption {
	return func(h *ThirdPartyHandlers) {
		h.userResolver = resolver
	}
}

func NewThirdPartyHandlers(accounts thirdPartyAccountService, accountValidator thirdPartyCredentialValidator, qrLogin thirdPartyQRCodeLoginService, monitors thirdPartyMonitorService, transport http.RoundTripper, options ...ThirdPartyHandlersOption) *ThirdPartyHandlers {
	if transport == nil {
		transport = http.DefaultTransport
	}
	handler := &ThirdPartyHandlers{
		accounts:         accounts,
		accountValidator: accountValidator,
		qrLogin:          qrLogin,
		monitors:         monitors,
		mediaClient:      &http.Client{Transport: transport, Timeout: 20 * time.Second},
	}
	for _, option := range options {
		if option != nil {
			option(handler)
		}
	}
	return handler
}

func NewModule(deps ModuleDeps) *ThirdPartyHandlers {
	options := []ThirdPartyHandlersOption{}
	if deps.UserResolver != nil {
		options = append(options, WithUserResolver(deps.UserResolver))
	}
	return NewThirdPartyHandlers(deps.Accounts, deps.AccountValidator, deps.QRLogin, deps.Monitors, deps.Transport, options...)
}
