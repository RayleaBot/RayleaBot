package douyin

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
	if !HasLoginCookie(cookies) {
		err := fmt.Errorf("douyin cookie missing login state")
		return thirdparty.AccountProfile{}, v.invalidStatus(err.Error()), err
	}
	// Cookies exist → account is valid. Profile is a best-effort bonus.
	status := v.validStatus()
	profile, err := FetchAccountProfile(ctx, v.client, cookies)
	if err != nil {
		// Profile unavailable is not fatal — cookies are still valid.
		return thirdparty.AccountProfile{}, status, nil
	}
	return profile, status, nil
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
