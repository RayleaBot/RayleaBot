package thirdpartyapi

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/qrcode"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

type ThirdPartyHandlers struct {
	accounts         thirdPartyAccountService
	accountValidator thirdPartyCredentialValidator
	qrLogin          thirdPartyQRCodeLoginService
}

type ModuleDeps struct {
	Accounts         thirdPartyAccountService
	AccountValidator thirdPartyCredentialValidator
	QRLogin          thirdPartyQRCodeLoginService
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

func NewThirdPartyHandlers(accounts thirdPartyAccountService, accountValidator thirdPartyCredentialValidator, qrLogin thirdPartyQRCodeLoginService) *ThirdPartyHandlers {
	return &ThirdPartyHandlers{
		accounts:         accounts,
		accountValidator: accountValidator,
		qrLogin:          qrLogin,
	}
}

func NewModule(deps ModuleDeps) *ThirdPartyHandlers {
	return NewThirdPartyHandlers(deps.Accounts, deps.AccountValidator, deps.QRLogin)
}
