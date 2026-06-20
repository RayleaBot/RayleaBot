package thirdpartylogin

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

type AccountValidator struct {
	client *http.Client
	now    func() time.Time
}

func NewAccountValidator(transport http.RoundTripper, now func() time.Time) *AccountValidator {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &AccountValidator{
		client: newHTTPClient(transport),
		now:    now,
	}
}

func (v *AccountValidator) CheckCookie(ctx context.Context, platform, cookie string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
	if v == nil {
		return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{}, fmt.Errorf("third-party account validator is unavailable")
	}
	normalized, err := thirdparty.NormalizePlatform(platform)
	if err != nil {
		return thirdparty.AccountProfile{}, v.invalidStatus(err.Error()), err
	}
	cookies := cookieMapFromHeader(cookie)
	switch normalized {
	case thirdparty.PlatformWeibo:
		return v.checkWeiboCookie(ctx, cookies)
	case thirdparty.PlatformDouyin:
		return v.checkDouyinCookie(cookies)
	case thirdparty.PlatformNeteaseMusic:
		return v.checkNeteaseMusicCookie(ctx, cookies)
	default:
		err := fmt.Errorf("unsupported third-party account platform %s", normalized)
		return thirdparty.AccountProfile{}, v.invalidStatus(err.Error()), err
	}
}

func (v *AccountValidator) checkWeiboCookie(ctx context.Context, cookies map[string]string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
	if !weiboHasLoginCookie(cookies) {
		err := fmt.Errorf("weibo cookie missing login state")
		return thirdparty.AccountProfile{}, v.invalidStatus(err.Error()), err
	}
	profile, err := fetchWeiboAccountProfile(ctx, v.client, cookies)
	if err != nil {
		return thirdparty.AccountProfile{}, v.validStatus(), nil
	}
	return profile, v.validStatus(), nil
}

func (v *AccountValidator) checkDouyinCookie(cookies map[string]string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
	if !douyinHasLoginCookie(cookies) {
		err := fmt.Errorf("douyin cookie missing login state")
		return thirdparty.AccountProfile{}, v.invalidStatus(err.Error()), err
	}
	return thirdparty.AccountProfile{}, v.validStatus(), nil
}

func (v *AccountValidator) checkNeteaseMusicCookie(ctx context.Context, cookies map[string]string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
	if !neteaseHasLoginCookie(cookies) {
		err := fmt.Errorf("netease music cookie missing login state")
		return thirdparty.AccountProfile{}, v.invalidStatus(err.Error()), err
	}
	profile, err := fetchNeteaseAccountProfile(ctx, v.client, cookies)
	if err != nil {
		return thirdparty.AccountProfile{}, v.validStatus(), nil
	}
	return profile, v.validStatus(), nil
}

func (v *AccountValidator) validStatus() thirdparty.CredentialStatus {
	checkedAt := v.now().UTC()
	return thirdparty.CredentialStatus{State: thirdparty.CredentialValid, CheckedAt: &checkedAt}
}

func (v *AccountValidator) invalidStatus(message string) thirdparty.CredentialStatus {
	checkedAt := v.now().UTC()
	return thirdparty.CredentialStatus{
		State:     thirdparty.CredentialInvalid,
		CheckedAt: &checkedAt,
		LastError: strings.TrimSpace(message),
	}
}
