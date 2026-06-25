package weibo

import (
	"context"
	"fmt"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"net/http"
	"strings"
	"time"
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
		err := fmt.Errorf("weibo cookie missing login state")
		return thirdparty.AccountProfile{}, v.invalidStatus(err.Error()), err
	}
	profile, err := FetchAccountProfile(ctx, v.client, cookies)
	if err != nil {
		// KEY FIX: Return unknown instead of valid when profile fetch fails.
		// The cookies are present (login succeeded) but profile retrieval failed.
		// This preserves the profile from QR login and prevents empty overwrite.
		return thirdparty.AccountProfile{}, v.unknownStatus(err.Error()), nil
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
