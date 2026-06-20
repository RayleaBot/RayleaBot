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

type fakeDouyinBrowser struct {
	createResult douyinBrowserCreateResult
	createErr    error
	pollResults  []douyinBrowserPollResult
	pollErr      error
	closed       []string
}

func (b *fakeDouyinBrowser) Create(context.Context, time.Time) (douyinBrowserCreateResult, error) {
	return b.createResult, b.createErr
}

func (b *fakeDouyinBrowser) Poll(context.Context, string) (douyinBrowserPollResult, error) {
	if b.pollErr != nil {
		return douyinBrowserPollResult{}, b.pollErr
	}
	if len(b.pollResults) == 0 {
		return douyinBrowserPollResult{State: StatePendingScan}, nil
	}
	result := b.pollResults[0]
	b.pollResults = b.pollResults[1:]
	return result, nil
}

func (b *fakeDouyinBrowser) Close(token string) {
	b.closed = append(b.closed, token)
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
		case "m.weibo.cn/api/config":
			return loginJSONResponse(request, http.StatusOK, `{"data":{"uid":"123456","screen_name":"微博用户","avatar_hd":"https://weibo.com/avatar.jpg"}}`)
		case "m.weibo.cn/api/container/getIndex", "weibo.com/ajax/profile/info":
			return loginJSONResponse(request, http.StatusOK, `{"data":{}}`)
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
	if succeeded.Account.UID != "123456" || succeeded.Account.Nickname != "微博用户" || succeeded.Account.AvatarURL == "" {
		t.Fatalf("unexpected weibo account profile: %#v", succeeded.Account)
	}
}

func TestServiceDouyinQRCodeLoginStatesAndCookie(t *testing.T) {
	now := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	browser := &fakeDouyinBrowser{
		createResult: douyinBrowserCreateResult{
			Token:     "douyin-token",
			QRCodeURL: "https://api.amemv.com/ucenter_web/app/aweme/scan_login/index?token=douyin-token",
			ExpiresAt: now.Add(3 * time.Minute),
		},
		pollResults: []douyinBrowserPollResult{
			{State: StatePendingConfirm},
			{
				State:  StateSucceeded,
				Cookie: "sessionid=fixture; sid_guard=fixture;",
				Cookies: map[string]string{
					"sessionid": "fixture",
					"sid_guard": "fixture",
				},
			},
		},
	}
	service := NewServiceWithOptions(Options{
		Now:           func() time.Time { return now },
		douyinBrowser: browser,
	})

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
	if len(browser.closed) != 1 || browser.closed[0] != "douyin-token" {
		t.Fatalf("closed douyin tokens = %#v, want douyin-token", browser.closed)
	}
}

func TestDouyinStatusValues(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		`"new"`:       "new",
		`"scanned"`:   "scanned",
		`"confirmed"`: "confirmed",
		`"expired"`:   "expired",
		`1`:           "1",
		`2`:           "2",
		`3`:           "3",
	}
	for raw, want := range cases {
		if got := douyinStatus([]byte(raw)); got != want {
			t.Fatalf("douyinStatus(%s) = %q, want %q", raw, got, want)
		}
	}
}

func TestDouyinQRCodeHTTPFlowUsesSSOParameters(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	provider := newDouyinProvider(newHTTPClient(loginRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host + request.URL.Path {
		case "sso.douyin.com/get_qrcode/":
			if request.URL.Query().Get("aid") != douyinAid || request.URL.Query().Get("service") != douyinServiceURL {
				return nil, fmt.Errorf("unexpected douyin create query: %s", request.URL.RawQuery)
			}
			return loginJSONResponse(request, http.StatusOK, `{"data":{"token":"douyin-token","qrcode_index_url":"https://api.amemv.com/scan?token=douyin-token"}}`, &http.Cookie{Name: "ttwid", Value: "seed"})
		case "sso.douyin.com/check_qrconnect/":
			if request.URL.Query().Get("aid") != douyinAid || request.URL.Query().Get("service") != douyinServiceURL || request.URL.Query().Get("token") != "douyin-token" {
				return nil, fmt.Errorf("unexpected douyin poll query: %s", request.URL.RawQuery)
			}
			return loginJSONResponse(request, http.StatusOK, `{"data":{"status":3,"redirect_url":"https://www.douyin.com/passport/sso/login/callback/?ticket=douyin-ticket"}}`)
		case "www.douyin.com/passport/sso/login/callback/":
			if request.URL.Query().Get("ticket") != "douyin-ticket" {
				return nil, fmt.Errorf("unexpected douyin callback query: %s", request.URL.RawQuery)
			}
			return loginJSONResponse(request, http.StatusOK, `{}`, &http.Cookie{Name: "sessionid", Value: "fixture"})
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	})), nil)

	session, err := provider.Create(context.Background(), now)
	if err != nil {
		t.Fatalf("create douyin qrcode: %v", err)
	}
	if session.Token != "douyin-token" || !strings.Contains(session.QRCodeURL, "douyin-token") || session.Cookies["ttwid"] != "seed" {
		t.Fatalf("unexpected douyin create session: %#v", session)
	}
	succeeded, err := provider.Poll(context.Background(), session, now)
	if err != nil {
		t.Fatalf("poll douyin qrcode: %v", err)
	}
	if succeeded.State != StateSucceeded || !strings.Contains(succeeded.Cookie, "sessionid=fixture;") {
		t.Fatalf("unexpected douyin success session: %#v", succeeded)
	}
}

func TestServiceDouyinQRCodeLoginRequiresLoginCookie(t *testing.T) {
	now := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	browser := &fakeDouyinBrowser{
		createResult: douyinBrowserCreateResult{
			Token:     "douyin-token",
			QRCodeURL: "https://api.amemv.com/ucenter_web/app/aweme/scan_login/index?token=douyin-token",
			ExpiresAt: now.Add(3 * time.Minute),
		},
		pollResults: []douyinBrowserPollResult{
			{
				State:  StateSucceeded,
				Cookie: "passport_csrf_token=fixture; ttwid=fixture;",
				Cookies: map[string]string{
					"passport_csrf_token": "fixture",
					"ttwid":               "fixture",
				},
			},
		},
	}
	service := NewServiceWithOptions(Options{
		Now:           func() time.Time { return now },
		douyinBrowser: browser,
	})

	create, err := service.Create(context.Background(), thirdparty.PlatformDouyin)
	if err != nil {
		t.Fatalf("create douyin qrcode login: %v", err)
	}
	if _, err := service.Poll(context.Background(), thirdparty.PlatformDouyin, create.LoginID); err == nil {
		t.Fatal("poll succeeded douyin qrcode login without login cookie returned nil error")
	}
}

func TestServiceNeteaseMusicQRCodeLoginStatesCookieAndAccount(t *testing.T) {
	now := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	checks := 0
	service := NewService(loginRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host + request.URL.Path {
		case "music.163.com/weapi/login/qrcode/unikey":
			if _, err := request.Cookie("deviceId"); err != nil {
				return nil, fmt.Errorf("missing netease deviceId cookie")
			}
			if cookie, err := request.Cookie("os"); err != nil || cookie.Value != "pc" {
				return nil, fmt.Errorf("missing netease os cookie")
			}
			return loginJSONResponse(request, http.StatusOK, `{"code":200,"unikey":"netease-key"}`)
		case "music.163.com/weapi/login/qrcode/client/login":
			if _, err := request.Cookie("deviceId"); err != nil {
				return nil, fmt.Errorf("missing netease poll deviceId cookie")
			}
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
	if !strings.Contains(create.QRCodeURL, "chainId=v1_") || !strings.Contains(create.QRCodeURL, "_web_login_") {
		t.Fatalf("qrcode url missing chainId: %s", create.QRCodeURL)
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

func TestServiceNeteaseMusicQRCodeLoginUsesPendingProfileOnSuccess(t *testing.T) {
	now := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	checks := 0
	service := NewService(loginRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host + request.URL.Path {
		case "music.163.com/weapi/login/qrcode/unikey":
			return loginJSONResponse(request, http.StatusOK, `{"code":200,"unikey":"netease-key"}`)
		case "music.163.com/weapi/login/qrcode/client/login":
			checks++
			if checks == 1 {
				return loginJSONResponse(request, http.StatusOK, `{"code":802,"profile":{"userId":123456789,"nickname":"待确认账号","avatarUrl":"https://p1.music.126.net/avatar.jpg"}}`)
			}
			return loginJSONResponse(request, http.StatusOK, `{"code":803}`, &http.Cookie{Name: "MUSIC_U", Value: "fixture"})
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	}), func() time.Time { return now })

	create, err := service.Create(context.Background(), thirdparty.PlatformNeteaseMusic)
	if err != nil {
		t.Fatalf("create netease music qrcode login: %v", err)
	}
	pending, err := service.Poll(context.Background(), thirdparty.PlatformNeteaseMusic, create.LoginID)
	if err != nil {
		t.Fatalf("poll pending netease music qrcode login: %v", err)
	}
	if pending.State != StatePendingConfirm || pending.Account.Nickname != "待确认账号" {
		t.Fatalf("unexpected pending response: %#v", pending)
	}
	succeeded, err := service.Poll(context.Background(), thirdparty.PlatformNeteaseMusic, create.LoginID)
	if err != nil {
		t.Fatalf("poll succeeded netease music qrcode login: %v", err)
	}
	if succeeded.State != StateSucceeded || !strings.Contains(succeeded.Cookie, "MUSIC_U=fixture;") {
		t.Fatalf("unexpected succeeded response: %#v", succeeded)
	}
	if succeeded.Account.UID != "123456789" || succeeded.Account.Nickname != "待确认账号" || succeeded.Account.AvatarURL == "" {
		t.Fatalf("unexpected succeeded account: %#v", succeeded.Account)
	}
}

func TestServiceWeiboQRCodeLoginIgnoresPartialCookieExchangeFailure(t *testing.T) {
	now := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	service := NewService(loginRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host + request.URL.Path {
		case "passport.weibo.com/sso/signin":
			return loginJSONResponse(request, http.StatusOK, `{}`, &http.Cookie{Name: "X-CSRF-TOKEN", Value: "csrf"})
		case "passport.weibo.com/sso/v2/qrcode/image":
			return loginJSONResponse(request, http.StatusOK, `{"retcode":20000000,"data":{"qrid":"weibo-token","image":"https://passport.weibo.com/qr"}}`)
		case "passport.weibo.com/sso/v2/qrcode/check":
			return loginJSONResponse(request, http.StatusOK, `{"retcode":20000000,"data":{"url":"https://passport.weibo.com/cross","alt":"alt-token"}}`)
		case "passport.weibo.com/cross":
			return loginJSONResponse(request, http.StatusBadGateway, `{}`)
		case "login.sina.com.cn/sso/login.php":
			return loginJSONResponse(request, http.StatusOK, `{}`, &http.Cookie{Name: "SUB", Value: "fixture"})
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	}), func() time.Time { return now })

	create, err := service.Create(context.Background(), thirdparty.PlatformWeibo)
	if err != nil {
		t.Fatalf("create weibo qrcode login: %v", err)
	}
	succeeded, err := service.Poll(context.Background(), thirdparty.PlatformWeibo, create.LoginID)
	if err != nil {
		t.Fatalf("poll succeeded weibo qrcode login: %v", err)
	}
	if succeeded.State != StateSucceeded || !strings.Contains(succeeded.Cookie, "SUB=fixture;") {
		t.Fatalf("unexpected succeeded response: %#v", succeeded)
	}
}

func TestServiceWeiboQRCodeLoginRequiresLoginCookie(t *testing.T) {
	now := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	service := NewService(loginRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host + request.URL.Path {
		case "passport.weibo.com/sso/signin":
			return loginJSONResponse(request, http.StatusOK, `{}`, &http.Cookie{Name: "X-CSRF-TOKEN", Value: "csrf"})
		case "passport.weibo.com/sso/v2/qrcode/image":
			return loginJSONResponse(request, http.StatusOK, `{"retcode":20000000,"data":{"qrid":"weibo-token","image":"https://passport.weibo.com/qr"}}`)
		case "passport.weibo.com/sso/v2/qrcode/check":
			return loginJSONResponse(request, http.StatusOK, `{"retcode":20000000,"data":{"url":"https://passport.weibo.com/cross"}}`)
		case "passport.weibo.com/cross":
			return loginJSONResponse(request, http.StatusOK, `{}`, &http.Cookie{Name: "X-CSRF-TOKEN", Value: "csrf"})
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	}), func() time.Time { return now })

	create, err := service.Create(context.Background(), thirdparty.PlatformWeibo)
	if err != nil {
		t.Fatalf("create weibo qrcode login: %v", err)
	}
	if _, err := service.Poll(context.Background(), thirdparty.PlatformWeibo, create.LoginID); err == nil {
		t.Fatal("poll succeeded without weibo login cookie")
	}
}

func TestServiceNeteaseMusicQRCodeLoginBodyCookieWithoutProfile(t *testing.T) {
	now := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	service := NewService(loginRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host + request.URL.Path {
		case "music.163.com/weapi/login/qrcode/unikey":
			return loginJSONResponse(request, http.StatusOK, `{"code":200,"unikey":"netease-key"}`)
		case "music.163.com/weapi/login/qrcode/client/login":
			return loginJSONResponse(request, http.StatusOK, `{"code":803,"cookie":"MUSIC_U=fixture; __csrf=fixture;"}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	}), func() time.Time { return now })

	create, err := service.Create(context.Background(), thirdparty.PlatformNeteaseMusic)
	if err != nil {
		t.Fatalf("create netease music qrcode login: %v", err)
	}
	succeeded, err := service.Poll(context.Background(), thirdparty.PlatformNeteaseMusic, create.LoginID)
	if err != nil {
		t.Fatalf("poll succeeded netease music qrcode login: %v", err)
	}
	if succeeded.State != StateSucceeded || succeeded.Cookie != "MUSIC_U=fixture; __csrf=fixture;" {
		t.Fatalf("unexpected succeeded response: %#v", succeeded)
	}
	if succeeded.Account != (thirdparty.AccountProfile{}) {
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
