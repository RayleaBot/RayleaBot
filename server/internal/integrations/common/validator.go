package common

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

type AccountValidator struct {
	Client     *http.Client
	Now        func() time.Time
	CheckFuncs map[string]func(context.Context, *http.Client, map[string]string) (thirdparty.AccountProfile, error)
}

func NewAccountValidator(transport http.RoundTripper, now func() time.Time) *AccountValidator {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &AccountValidator{
		Client:     NewHTTPClient(transport),
		Now:        now,
		CheckFuncs: make(map[string]func(context.Context, *http.Client, map[string]string) (thirdparty.AccountProfile, error)),
	}
}

func (v *AccountValidator) RegisterPlatform(platform string, checkFn func(context.Context, *http.Client, map[string]string) (thirdparty.AccountProfile, error)) {
	if v == nil {
		return
	}
	v.CheckFuncs[platform] = checkFn
}

func (v *AccountValidator) CheckCookie(ctx context.Context, platform, cookie string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
	if v == nil {
		return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{}, fmt.Errorf("third-party account validator is unavailable")
	}
	normalized, err := thirdparty.NormalizePlatform(platform)
	if err != nil {
		return thirdparty.AccountProfile{}, v.invalidStatus(err.Error()), err
	}
	cookies := CookieMapFromHeader(cookie)

	checkFn := v.CheckFuncs[normalized]
	if checkFn == nil {
		err := fmt.Errorf("unsupported third-party account platform %s", normalized)
		return thirdparty.AccountProfile{}, v.invalidStatus(err.Error()), err
	}

	profile, err := checkFn(ctx, v.Client, cookies)
	if err != nil {
		// Profile fetch failed but login cookies are present.
		// Return nil error so the caller preserves the QR-login profile
		// and marks the credential as unknown (not invalid).
		return thirdparty.AccountProfile{}, v.unknownStatus(err.Error()), nil
	}
	return profile, v.validStatus(), nil
}

func (v *AccountValidator) validStatus() thirdparty.CredentialStatus {
	checkedAt := v.Now().UTC()
	return thirdparty.CredentialStatus{State: thirdparty.CredentialValid, CheckedAt: &checkedAt}
}

func (v *AccountValidator) invalidStatus(message string) thirdparty.CredentialStatus {
	checkedAt := v.Now().UTC()
	return thirdparty.CredentialStatus{
		State:     thirdparty.CredentialInvalid,
		CheckedAt: &checkedAt,
		LastError: strings.TrimSpace(message),
	}
}

func (v *AccountValidator) unknownStatus(message string) thirdparty.CredentialStatus {
	checkedAt := v.Now().UTC()
	return thirdparty.CredentialStatus{
		State:     thirdparty.CredentialUnknown,
		CheckedAt: &checkedAt,
		LastError: strings.TrimSpace(message),
	}
}
