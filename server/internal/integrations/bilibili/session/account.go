package session

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

const navURL = "https://api.bilibili.com/x/web-interface/nav"

type AccountClient struct {
	client   *http.Client
	identity *IdentityProvider
	now      func() time.Time
}

func NewAccountClient(transport http.RoundTripper, now func() time.Time, identity *IdentityProvider) *AccountClient {
	if transport == nil {
		transport = http.DefaultTransport
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	if identity == nil {
		identity = NewIdentityProvider(now)
	}
	return &AccountClient{
		client:   &http.Client{Transport: transport, Timeout: DefaultRequestTimeout},
		identity: identity,
		now:      now,
	}
}

func (c *AccountClient) CheckCookie(ctx context.Context, cookie string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
	checkedAt := c.now().UTC()
	if err := validateCookieForLogin(cookie); err != nil {
		return thirdparty.AccountProfile{}, thirdparty.CheckedCredential(thirdparty.PlatformBilibili, checkedAt, thirdparty.ErrorAuth), err
	}
	profile, err := c.fetchNav(ctx, cookie)
	var kind thirdparty.ErrorKind
	if err != nil {
		kind = thirdparty.ErrorUpstream
		if rejected := AsError(err); rejected != nil && rejected.Kind == ErrorAuth &&
			(rejected.HTTPStatus == 0 || rejected.HTTPStatus == http.StatusUnauthorized || rejected.Code == -101 || rejected.Code == -102 || rejected.Code == -658) {
			kind = thirdparty.ErrorAuth
		}
	}
	return profile, thirdparty.CheckedCredential(thirdparty.PlatformBilibili, checkedAt, kind), err
}

func (c *AccountClient) fetchNav(ctx context.Context, cookie string) (thirdparty.AccountProfile, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, navURL, nil)
	if err != nil {
		return thirdparty.AccountProfile{}, err
	}
	c.identity.ApplyHeaders(request, http.MethodGet)
	request.Header.Set("Cookie", strings.TrimSpace(cookie))

	response, err := c.client.Do(request)
	if err != nil {
		return thirdparty.AccountProfile{}, err
	}
	defer func(release func() error) { _ = release() }(response.Body.Close)
	var document struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			IsLogin bool   `json:"isLogin"`
			Mid     any    `json:"mid"`
			UName   string `json:"uname"`
			Face    string `json:"face"`
		} `json:"data"`
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return thirdparty.AccountProfile{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return thirdparty.AccountProfile{}, &Error{Kind: classifyHTTPStatus(response.StatusCode), HTTPStatus: response.StatusCode, Message: responseExcerpt(body)}
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return thirdparty.AccountProfile{}, &Error{Kind: ErrorInvalidResponse, HTTPStatus: response.StatusCode, Message: responseExcerpt(body), Err: err}
	}
	if document.Code != 0 || !document.Data.IsLogin {
		message := strings.TrimSpace(document.Message)
		if message == "" {
			message = "账号未登录"
		}
		return thirdparty.AccountProfile{}, apiError(response.StatusCode, document.Code, message, body)
	}
	uid := strings.TrimSpace(stringValue(document.Data.Mid))
	if uid == "" {
		return thirdparty.AccountProfile{}, fmt.Errorf("bilibili nav missing uid")
	}
	return thirdparty.AccountProfile{
		UID:       uid,
		Nickname:  strings.TrimSpace(document.Data.UName),
		AvatarURL: normalizeURL(document.Data.Face),
	}, nil
}
