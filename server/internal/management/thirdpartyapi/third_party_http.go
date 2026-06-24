package thirdpartyapi

import (
	"context"
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	thirdpartylogin "github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdpartylogin"
)

type ThirdPartyHandlers struct {
	accounts                 thirdPartyAccountService
	accountValidator         thirdPartyCredentialValidator
	platformAccountValidator *common.AccountValidator
	qrLogin                  thirdPartyQRCodeLoginService
	monitors                 thirdPartyMonitorService
	douyinUserResolver       douyinUserResolver
	mediaClient              *http.Client
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
	Create(context.Context, string) (common.CreateResult, error)
	Poll(context.Context, string, string) (common.PollResult, error)
}

type douyinUserResolver interface {
	ResolveUser(context.Context, string, []map[string]string) ([]thirdparty.AccountProfile, bool, error)
}

type ThirdPartyHandlersOption func(*ThirdPartyHandlers)

func WithDouyinUserResolver(resolver douyinUserResolver) ThirdPartyHandlersOption {
	return func(h *ThirdPartyHandlers) {
		h.douyinUserResolver = resolver
	}
}

func NewThirdPartyHandlers(accounts thirdPartyAccountService, accountValidator thirdPartyCredentialValidator, qrLogin thirdPartyQRCodeLoginService, monitors thirdPartyMonitorService, transport http.RoundTripper, options ...ThirdPartyHandlersOption) *ThirdPartyHandlers {
	if transport == nil {
		transport = http.DefaultTransport
	}
	handler := &ThirdPartyHandlers{
		accounts:                 accounts,
		accountValidator:         accountValidator,
		platformAccountValidator: thirdpartylogin.NewAccountValidator(transport, nil),
		qrLogin:                  qrLogin,
		monitors:                 monitors,
		mediaClient:              &http.Client{Transport: transport, Timeout: 20 * time.Second},
	}
	for _, option := range options {
		if option != nil {
			option(handler)
		}
	}
	return handler
}
