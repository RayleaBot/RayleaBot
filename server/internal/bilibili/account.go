package bilibili

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

const navURL = "https://api.bilibili.com/x/web-interface/nav"

type AccountClient struct {
	client *http.Client
	now    func() time.Time
}

func NewAccountClient(transport http.RoundTripper, now func() time.Time) *AccountClient {
	if transport == nil {
		transport = http.DefaultTransport
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &AccountClient{
		client: &http.Client{Transport: transport, Timeout: defaultRequestTimeout},
		now:    now,
	}
}

func (c *AccountClient) CheckCookie(ctx context.Context, cookie string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
	checkedAt := c.now().UTC()
	profile, err := c.fetchNav(ctx, cookie)
	if err != nil {
		return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{
			State:     thirdparty.CredentialInvalid,
			CheckedAt: &checkedAt,
			LastError: err.Error(),
		}, err
	}
	return profile, thirdparty.CredentialStatus{
		State:     thirdparty.CredentialValid,
		CheckedAt: &checkedAt,
		LastError: "",
	}, nil
}

func (c *AccountClient) fetchNav(ctx context.Context, cookie string) (thirdparty.AccountProfile, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, navURL, nil)
	if err != nil {
		return thirdparty.AccountProfile{}, err
	}
	request.Header.Set("Accept", "application/json, text/plain, */*")
	request.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	request.Header.Set("Referer", "https://www.bilibili.com/")
	request.Header.Set("Cookie", strings.TrimSpace(cookie))

	response, err := c.client.Do(request)
	if err != nil {
		return thirdparty.AccountProfile{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return thirdparty.AccountProfile{}, fmt.Errorf("bilibili nav http %d", response.StatusCode)
	}
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
	if err := json.Unmarshal(body, &document); err != nil {
		return thirdparty.AccountProfile{}, err
	}
	if document.Code != 0 || !document.Data.IsLogin {
		message := strings.TrimSpace(document.Message)
		if message == "" {
			message = "账号未登录"
		}
		return thirdparty.AccountProfile{}, fmt.Errorf("bilibili nav invalid: %s", message)
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
