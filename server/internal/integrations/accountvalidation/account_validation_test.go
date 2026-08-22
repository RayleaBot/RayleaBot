package accountvalidation

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestWeiboCredentialValidationMarksExplicitLogoutInvalid(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 1, 16, 16, 0, time.UTC)
	validator := NewDefault(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "https://m.weibo.cn/api/config":
			return jsonHTTPResponse(http.StatusOK, `{"ok":1,"data":{"login":false}}`), nil
		case "https://weibo.com/ajax/side/config":
			return jsonHTTPResponse(http.StatusOK, `{"ok":-100,"data":{}}`), nil
		default:
			return nil, errors.New("unexpected Weibo request: " + request.URL.String())
		}
	}), func() time.Time { return now })

	_, credential, err := validator.CheckCookie(context.Background(), thirdparty.PlatformWeibo, "SUB=expired;")
	if err == nil {
		t.Fatal("expected explicit logout error")
	}
	if credential.State != thirdparty.CredentialInvalid || credential.CheckedAt == nil || credential.LastError != "微博账号 CK 已失效，请重新扫码" {
		t.Fatalf("unexpected credential: %#v", credential)
	}
	if typed := thirdparty.AsThirdPartyError(err); typed == nil || typed.Kind != thirdparty.ErrorAuth {
		t.Fatalf("logout error kind = %#v", typed)
	}
}

func TestWeiboCredentialValidationLetsEitherExplicitLogoutSignalWin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mobileConfig string
		sideConfig   string
	}{
		{
			name:         "mobile login false",
			mobileConfig: `{"ok":1,"data":{"login":false}}`,
			sideConfig:   `{"ok":1,"data":{"uid":"123456","screen_name":"微博用户"}}`,
		},
		{
			name:         "side config minus one hundred",
			mobileConfig: `{"ok":1,"data":{"login":true,"uid":"123456","screen_name":"微博用户"}}`,
			sideConfig:   `{"ok":-100,"data":{}}`,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			validator := NewDefault(roundTripFunc(func(request *http.Request) (*http.Response, error) {
				switch request.URL.String() {
				case "https://m.weibo.cn/api/config":
					return jsonHTTPResponse(http.StatusOK, test.mobileConfig), nil
				case "https://weibo.com/ajax/side/config":
					return jsonHTTPResponse(http.StatusOK, test.sideConfig), nil
				default:
					return nil, errors.New("unexpected Weibo request: " + request.URL.String())
				}
			}), time.Now)

			_, credential, err := validator.CheckCookie(context.Background(), thirdparty.PlatformWeibo, "SUB=expired;")
			if err == nil || credential.State != thirdparty.CredentialInvalid || credential.LastError != "微博账号 CK 已失效，请重新扫码" {
				t.Fatalf("explicit logout result: credential=%#v err=%v", credential, err)
			}
		})
	}
}

func TestWeiboCredentialValidationKeepsHTTP432Unknown(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 1, 16, 16, 0, time.UTC)
	validator := NewDefault(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonHTTPResponse(432, ""), nil
	}), func() time.Time { return now })

	_, credential, err := validator.CheckCookie(context.Background(), thirdparty.PlatformWeibo, "SUB=fixture;")
	if err == nil {
		t.Fatal("expected HTTP 432 error")
	}
	if credential.State != thirdparty.CredentialUnknown || credential.CheckedAt == nil || credential.LastError != "微博 CK 状态暂时无法确认，请稍后重试" {
		t.Fatalf("unexpected credential: %#v", credential)
	}
	typed := thirdparty.AsThirdPartyError(err)
	if typed == nil || typed.Kind != thirdparty.ErrorUpstream || typed.HTTPStatus != 432 {
		t.Fatalf("HTTP 432 error = %#v", typed)
	}
}

func TestWeiboCredentialValidationKeepsNetworkFailureUnknown(t *testing.T) {
	t.Parallel()

	validator := NewDefault(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network unavailable")
	}), time.Now)

	_, credential, err := validator.CheckCookie(context.Background(), thirdparty.PlatformWeibo, "SUB=fixture;")
	if err == nil || credential.State != thirdparty.CredentialUnknown {
		t.Fatalf("network validation result: credential=%#v err=%v", credential, err)
	}
	if typed := thirdparty.AsThirdPartyError(err); typed == nil || typed.Kind != thirdparty.ErrorNetwork {
		t.Fatalf("network error kind = %#v", typed)
	}
}

func TestWeiboCredentialValidationAcceptsProfile(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 1, 16, 16, 0, time.UTC)
	validator := NewDefault(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "https://m.weibo.cn/api/config":
			return jsonHTTPResponse(http.StatusOK, `{"ok":1,"data":{"login":true,"uid":"123456","screen_name":"微博用户","avatar_hd":"https://tvax1.sinaimg.cn/avatar.jpg"}}`), nil
		case "https://weibo.com/ajax/side/config":
			return jsonHTTPResponse(http.StatusOK, `{"ok":1,"data":{}}`), nil
		case "https://m.weibo.cn/":
			return jsonHTTPResponse(http.StatusOK, "<html></html>"), nil
		default:
			if strings.HasPrefix(request.URL.String(), "https://m.weibo.cn/api/container/getIndex?") || strings.HasPrefix(request.URL.String(), "https://weibo.com/ajax/profile/info?") {
				return jsonHTTPResponse(http.StatusOK, `{"data":{}}`), nil
			}
			return nil, errors.New("unexpected Weibo request: " + request.URL.String())
		}
	}), func() time.Time { return now })

	profile, credential, err := validator.CheckCookie(context.Background(), thirdparty.PlatformWeibo, "SUB=fixture;")
	if err != nil {
		t.Fatalf("CheckCookie returned error: %v", err)
	}
	if profile.UID != "123456" || profile.Nickname != "微博用户" || profile.AvatarURL != "https://tvax1.sinaimg.cn/avatar.jpg" {
		t.Fatalf("unexpected profile: %#v", profile)
	}
	if credential.State != thirdparty.CredentialValid || credential.CheckedAt == nil || !credential.CheckedAt.Equal(now) {
		t.Fatalf("unexpected credential: %#v", credential)
	}
}

func TestBilibiliCredentialValidationKeepsNetworkFailureUnknown(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 1, 16, 16, 0, time.UTC)
	validator := NewDefault(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network unavailable")
	}), func() time.Time { return now })

	_, credential, err := validator.CheckCookie(context.Background(), thirdparty.PlatformBilibili, "SESSDATA=fixture;")
	if err == nil {
		t.Fatal("expected network error")
	}
	if credential.State != thirdparty.CredentialUnknown || credential.CheckedAt == nil || credential.LastError != "Bilibili CK 状态暂时无法确认，请稍后重试" {
		t.Fatalf("network credential = %#v", credential)
	}
}

func TestBilibiliCredentialValidationMarksExplicitLogoutInvalid(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 1, 16, 16, 0, time.UTC)
	validator := NewDefault(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonHTTPResponse(http.StatusOK, `{"code":-101,"message":"账号未登录","data":{"isLogin":false}}`), nil
	}), func() time.Time { return now })

	_, credential, err := validator.CheckCookie(context.Background(), thirdparty.PlatformBilibili, "SESSDATA=expired;")
	if err == nil {
		t.Fatal("expected explicit logout error")
	}
	if credential.State != thirdparty.CredentialInvalid || credential.CheckedAt == nil || credential.LastError != "Bilibili 账号 CK 已失效，请重新扫码" {
		t.Fatalf("logout credential = %#v", credential)
	}
}

func jsonHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
