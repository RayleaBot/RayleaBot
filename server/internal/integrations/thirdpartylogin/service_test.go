package thirdpartylogin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

type loginRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn loginRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestServiceWeiboQRCodeLoginStatesAndCookie(t *testing.T) {
	now := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	checks := 0
	service := NewService(loginRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host + request.URL.Path {
		case "passport.weibo.com/sso/signin":
			return loginJSONResponse(request, http.StatusOK, `{}`, &http.Cookie{Name: "X-CSRF-TOKEN", Value: "csrf"})
		case "passport.weibo.com/sso/v2/qrcode/image":
			if request.Header.Get("x-csrf-token") != "csrf" {
				return nil, fmt.Errorf("missing weibo csrf header")
			}
			return loginJSONResponse(request, http.StatusOK, `{"retcode":20000000,"data":{"qrid":"weibo-token","image":"https://passport.weibo.com/qr?data=https%3A%2F%2Fpassport.weibo.cn%2Fsignin%2Fqrcode%2Fscan%3Fqr%3Dweibo-token"}}`)
		case "passport.weibo.com/sso/v2/qrcode/check":
			checks++
			if checks == 1 {
				return loginJSONResponse(request, http.StatusOK, `{"retcode":50114002,"data":{}}`)
			}
			return loginJSONResponse(request, http.StatusOK, `{"retcode":20000000,"data":{"url":"https://passport.weibo.com/cross"}}`)
		case "passport.weibo.com/cross":
			return loginRedirectResponse(request, "https://weibo.com/u/1", &http.Cookie{Name: "SUB", Value: "fixture"})
		case "weibo.com/u/1":
			return loginJSONResponse(request, http.StatusOK, `{}`, &http.Cookie{Name: "SUBP", Value: "fixture"})
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	}), func() time.Time { return now })

	create, err := service.Create(context.Background(), thirdparty.PlatformWeibo)
	if err != nil {
		t.Fatalf("create weibo qrcode login: %v", err)
	}
	if create.Platform != thirdparty.PlatformWeibo || create.State != StatePendingScan || !strings.Contains(create.QRCodeURL, "weibo-token") {
		t.Fatalf("unexpected create response: %#v", create)
	}

	pending, err := service.Poll(context.Background(), thirdparty.PlatformWeibo, create.LoginID)
	if err != nil {
		t.Fatalf("poll pending weibo qrcode login: %v", err)
	}
	if pending.State != StatePendingConfirm || pending.Cookie != "" {
		t.Fatalf("unexpected pending response: %#v", pending)
	}

	succeeded, err := service.Poll(context.Background(), thirdparty.PlatformWeibo, create.LoginID)
	if err != nil {
		t.Fatalf("poll succeeded weibo qrcode login: %v", err)
	}
	if succeeded.State != StateSucceeded || !strings.Contains(succeeded.Cookie, "SUB=fixture;") || !strings.Contains(succeeded.Cookie, "SUBP=fixture;") {
		t.Fatalf("unexpected succeeded response: %#v", succeeded)
	}
}

func TestServiceDouyinQRCodeLoginStatesAndCookie(t *testing.T) {
	now := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	checks := 0
	service := NewService(loginRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host + request.URL.Path {
		case "sso.douyin.com/get_qrcode/":
			return loginJSONResponse(request, http.StatusOK, `{"error_code":0,"data":{"qrcode":"https://sso.douyin.com/scan?token=douyin-token","token":"douyin-token"}}`)
		case "sso.douyin.com/check_qrconnect/":
			checks++
			if checks == 1 {
				return loginJSONResponse(request, http.StatusOK, `{"error_code":0,"data":{"error_code":0,"status":2}}`)
			}
			return loginJSONResponse(request, http.StatusOK, `{"error_code":0,"data":{"error_code":0,"status":3,"redirect_url":"https://www.douyin.com/passport/sso/login/callback/?ticket=douyin-ticket"}}`)
		case "www.douyin.com/passport/sso/login/callback/":
			return loginJSONResponse(request, http.StatusOK, `{}`, &http.Cookie{Name: "sessionid", Value: "fixture"}, &http.Cookie{Name: "sid_guard", Value: "fixture"})
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	}), func() time.Time { return now })

	create, err := service.Create(context.Background(), thirdparty.PlatformDouyin)
	if err != nil {
		t.Fatalf("create douyin qrcode login: %v", err)
	}
	if create.Platform != thirdparty.PlatformDouyin || create.State != StatePendingScan || !strings.Contains(create.QRCodeURL, "douyin-token") {
		t.Fatalf("unexpected create response: %#v", create)
	}

	pending, err := service.Poll(context.Background(), thirdparty.PlatformDouyin, create.LoginID)
	if err != nil {
		t.Fatalf("poll pending douyin qrcode login: %v", err)
	}
	if pending.State != StatePendingConfirm || pending.Cookie != "" {
		t.Fatalf("unexpected pending response: %#v", pending)
	}

	succeeded, err := service.Poll(context.Background(), thirdparty.PlatformDouyin, create.LoginID)
	if err != nil {
		t.Fatalf("poll succeeded douyin qrcode login: %v", err)
	}
	if succeeded.State != StateSucceeded || !strings.Contains(succeeded.Cookie, "sessionid=fixture;") || !strings.Contains(succeeded.Cookie, "sid_guard=fixture;") {
		t.Fatalf("unexpected succeeded response: %#v", succeeded)
	}
}

func TestServiceNeteaseMusicQRCodeLoginStatesCookieAndAccount(t *testing.T) {
	now := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	checks := 0
	service := NewService(loginRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host + request.URL.Path {
		case "music.163.com/weapi/login/qrcode/unikey":
			return loginJSONResponse(request, http.StatusOK, `{"code":200,"unikey":"netease-key"}`)
		case "music.163.com/weapi/login/qrcode/client/login":
			checks++
			if checks == 1 {
				return loginJSONResponse(request, http.StatusOK, `{"code":802}`)
			}
			return loginJSONResponse(
				request,
				http.StatusOK,
				`{"code":803,"profile":{"userId":123456789,"nickname":"网易云音乐账号","avatarUrl":"https://p1.music.126.net/avatar.jpg"}}`,
				&http.Cookie{Name: "MUSIC_U", Value: "fixture"},
				&http.Cookie{Name: "__csrf", Value: "fixture"},
			)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	}), func() time.Time { return now })

	create, err := service.Create(context.Background(), thirdparty.PlatformNeteaseMusic)
	if err != nil {
		t.Fatalf("create netease music qrcode login: %v", err)
	}
	if create.Platform != thirdparty.PlatformNeteaseMusic || create.State != StatePendingScan || !strings.Contains(create.QRCodeURL, "netease-key") {
		t.Fatalf("unexpected create response: %#v", create)
	}

	pending, err := service.Poll(context.Background(), thirdparty.PlatformNeteaseMusic, create.LoginID)
	if err != nil {
		t.Fatalf("poll pending netease music qrcode login: %v", err)
	}
	if pending.State != StatePendingConfirm || pending.Cookie != "" {
		t.Fatalf("unexpected pending response: %#v", pending)
	}

	succeeded, err := service.Poll(context.Background(), thirdparty.PlatformNeteaseMusic, create.LoginID)
	if err != nil {
		t.Fatalf("poll succeeded netease music qrcode login: %v", err)
	}
	if succeeded.State != StateSucceeded || !strings.Contains(succeeded.Cookie, "MUSIC_U=fixture;") || !strings.Contains(succeeded.Cookie, "__csrf=fixture;") {
		t.Fatalf("unexpected succeeded response: %#v", succeeded)
	}
	if succeeded.Account.UID != "123456789" || succeeded.Account.Nickname != "网易云音乐账号" || succeeded.Account.AvatarURL == "" {
		t.Fatalf("unexpected succeeded account: %#v", succeeded.Account)
	}
}

func TestServiceQRCodeLoginExpiredAndUnknownLoginID(t *testing.T) {
	now := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	service := NewService(loginRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host + request.URL.Path {
		case "music.163.com/weapi/login/qrcode/unikey":
			return loginJSONResponse(request, http.StatusOK, `{"code":200,"unikey":"netease-key"}`)
		default:
			return nil, fmt.Errorf("unexpected request after create: %s %s", request.Method, request.URL.String())
		}
	}), func() time.Time { return now })

	create, err := service.Create(context.Background(), thirdparty.PlatformNeteaseMusic)
	if err != nil {
		t.Fatalf("create netease music qrcode login: %v", err)
	}
	now = create.ExpiresAt.Add(time.Second)
	expired, err := service.Poll(context.Background(), thirdparty.PlatformNeteaseMusic, create.LoginID)
	if err != nil {
		t.Fatalf("poll expired qrcode login: %v", err)
	}
	if expired.State != StateExpired {
		t.Fatalf("state = %q, want %q", expired.State, StateExpired)
	}
	if _, err := service.Poll(context.Background(), thirdparty.PlatformNeteaseMusic, "missing-login-id"); err != ErrLoginSessionNotFound {
		t.Fatalf("unknown login err = %v, want %v", err, ErrLoginSessionNotFound)
	}
}

func TestServiceQRCodeLoginRemoteAndUnsupportedErrors(t *testing.T) {
	service := NewService(loginRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return loginJSONResponse(request, http.StatusBadGateway, `{"error":"upstream"}`)
	}), func() time.Time { return time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC) })

	if _, err := service.Create(context.Background(), thirdparty.PlatformWeibo); err == nil {
		t.Fatal("create with remote error returned nil error")
	}
	if _, err := service.Create(context.Background(), thirdparty.PlatformBilibili); err != ErrUnsupportedPlatform {
		t.Fatalf("unsupported platform err = %v, want %v", err, ErrUnsupportedPlatform)
	}
}

func loginJSONResponse(request *http.Request, status int, body string, cookies ...*http.Cookie) (*http.Response, error) {
	header := http.Header{"Content-Type": []string{"application/json"}}
	for _, cookie := range cookies {
		header.Add("Set-Cookie", cookie.String())
	}
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}, nil
}

func loginRedirectResponse(request *http.Request, location string, cookies ...*http.Cookie) (*http.Response, error) {
	header := http.Header{"Location": []string{location}}
	for _, cookie := range cookies {
		header.Add("Set-Cookie", cookie.String())
	}
	return &http.Response{
		StatusCode: http.StatusFound,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader("")),
		Request:    request,
	}, nil
}
