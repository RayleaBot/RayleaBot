package weibo

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

type Validator struct {
	client *http.Client
	now    func() time.Time
}

func NewValidator(transport http.RoundTripper, now func() time.Time) *Validator {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Validator{
		client: thirdparty.NewHTTPClient(transport),
		now:    now,
	}
}

func (v *Validator) CheckCookie(ctx context.Context, cookies map[string]string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
	if !weiboHasLoginCookie(cookies) {
		err := weiboCredentialExpiredError(0, 0)
		return thirdparty.AccountProfile{}, v.invalidStatus("微博账号 CK 已失效，请重新扫码"), err
	}
	profile, err := FetchAccountProfile(ctx, v.client, cookies)
	if err != nil {
		if typed := thirdparty.AsThirdPartyError(err); typed != nil && (typed.Kind == thirdparty.ErrorAuth || typed.Kind == thirdparty.ErrorExpired) {
			return thirdparty.AccountProfile{}, v.invalidStatus("微博账号 CK 已失效，请重新扫码"), err
		}
		return thirdparty.AccountProfile{}, v.unknownStatus("微博 CK 状态暂时无法确认，请稍后重试"), err
	}
	return profile, v.validStatus(), nil
}

func (v *Validator) validStatus() thirdparty.CredentialStatus {
	checkedAt := v.now().UTC()
	return thirdparty.CredentialStatus{State: thirdparty.CredentialValid, CheckedAt: &checkedAt}
}

func (v *Validator) invalidStatus(message string) thirdparty.CredentialStatus {
	checkedAt := v.now().UTC()
	return thirdparty.CredentialStatus{
		State:     thirdparty.CredentialInvalid,
		CheckedAt: &checkedAt,
		LastError: strings.TrimSpace(message),
	}
}

func (v *Validator) unknownStatus(message string) thirdparty.CredentialStatus {
	checkedAt := v.now().UTC()
	return thirdparty.CredentialStatus{
		State:     thirdparty.CredentialUnknown,
		CheckedAt: &checkedAt,
		LastError: strings.TrimSpace(message),
	}
}
