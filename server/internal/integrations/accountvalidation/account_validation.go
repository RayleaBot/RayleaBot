package accountvalidation

import (
	"context"
	"net/http"
	"time"

	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/douyin"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/netease_music"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/weibo"
)

type Validator struct {
	bilibili *bilibilisession.AccountClient
	client   *http.Client
	now      func() time.Time
}

func NewDefault(transport http.RoundTripper, now func() time.Time) *Validator {
	if now == nil {
		now = time.Now
	}
	return &Validator{
		bilibili: bilibilisession.NewAccountClient(transport, now, nil),
		client:   thirdparty.NewHTTPClient(transport), now: now,
	}
}

func (v *Validator) CheckCookie(ctx context.Context, platform, cookie string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
	normalized, err := thirdparty.NormalizePlatform(platform)
	if err != nil {
		return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{}, err
	}
	if normalized == thirdparty.PlatformBilibili {
		return v.bilibili.CheckCookie(ctx, cookie)
	}
	cookies := thirdparty.CookieMapFromHeader(cookie)
	var profile thirdparty.AccountProfile
	switch normalized {
	case thirdparty.PlatformWeibo:
		profile, err = weibo.FetchAccountProfile(ctx, v.client, cookies)
	case thirdparty.PlatformDouyin:
		profile, err = douyin.FetchAccountProfile(ctx, v.client, cookies)
	case thirdparty.PlatformNeteaseMusic:
		profile, err = netease_music.FetchAccountProfile(ctx, v.client, cookies)
	}
	var kind thirdparty.ErrorKind
	if err != nil {
		kind = thirdparty.ErrorUpstream
		if platformError := thirdparty.AsThirdPartyError(err); platformError != nil {
			kind = platformError.Kind
		}
	}
	return profile, thirdparty.CheckedCredential(normalized, v.now(), kind), err
}
