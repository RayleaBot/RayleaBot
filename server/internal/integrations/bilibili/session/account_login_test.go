package session

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAccountClientCheckCookieReadsNavProfile(t *testing.T) {
	t.Parallel()

	client := NewAccountClient(bilibiliRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != navURL {
			return nil, fmt.Errorf("unexpected nav url: %s", request.URL.String())
		}
		if request.Header.Get("Cookie") != "SESSDATA=fixture; bili_jct=fixture;" {
			return nil, fmt.Errorf("unexpected cookie header: %q", request.Header.Get("Cookie"))
		}
		return bilibiliJSONResponse(`{
			"code": 0,
			"data": {
				"isLogin": true,
				"mid": 123456,
				"uname": "测试账号昵称",
				"face": "//i0.hdslb.com/bfs/face/test-account.jpg"
			}
		}`), nil
	}), func() time.Time { return time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC) }, nil)

	profile, credential, err := client.CheckCookie(context.Background(), "SESSDATA=fixture; bili_jct=fixture;")
	if err != nil {
		t.Fatalf("CheckCookie returned error: %v", err)
	}
	if profile.UID != "123456" || profile.Nickname != "测试账号昵称" || profile.AvatarURL != "https://i0.hdslb.com/bfs/face/test-account.jpg" {
		t.Fatalf("unexpected profile: %#v", profile)
	}
	if credential.State != "valid" || credential.CheckedAt == nil || credential.CheckedAt.Format(time.RFC3339) != "2026-06-08T08:00:00Z" || credential.LastError != "" {
		t.Fatalf("unexpected credential: %#v", credential)
	}
}

func TestAccountClientCheckCookieMarksInvalidNavResponse(t *testing.T) {
	t.Parallel()

	client := NewAccountClient(bilibiliRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return bilibiliJSONResponse(`{"code": -101, "message": "账号未登录", "data": {"isLogin": false}}`), nil
	}), func() time.Time { return time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC) }, nil)

	_, credential, err := client.CheckCookie(context.Background(), "SESSDATA=expired;")
	if err == nil {
		t.Fatal("expected invalid cookie error")
	}
	if credential.State != "invalid" || credential.CheckedAt == nil || !strings.Contains(credential.LastError, "账号未登录") {
		t.Fatalf("unexpected invalid credential: %#v", credential)
	}
}

func TestAccountClientCheckCookieKeepsRiskControlUnknown(t *testing.T) {
	t.Parallel()

	client := NewAccountClient(bilibiliRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return bilibiliJSONResponse(`{"code": -352, "message": "风控校验失败", "data": null}`), nil
	}), func() time.Time { return time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC) }, nil)

	_, credential, err := client.CheckCookie(context.Background(), "SESSDATA=fixture;")
	if err == nil {
		t.Fatal("expected risk control error")
	}
	if credential.State != "unknown" || credential.CheckedAt == nil || !strings.Contains(credential.LastError, "code -352") {
		t.Fatalf("unexpected risk-control credential: %#v", credential)
	}
}

func TestQRLoginServiceCreatePollAndReturnCookie(t *testing.T) {
	t.Parallel()

	pollCount := 0
	service := NewQRLoginService(bilibiliRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case qrCodeGenerateURL:
			return bilibiliJSONResponse(`{
				"code": 0,
				"data": {
					"url": "https://passport.bilibili.com/h5-app/passport/login/scan?navhide=1&qrcode_key=fixture-key",
					"qrcode_key": "fixture-key"
				}
			}`), nil
		case qrCodePollURL + "?qrcode_key=fixture-key&source=main-fe-header":
			pollCount += 1
			if pollCount == 1 {
				return bilibiliJSONResponse(`{"code":0,"data":{"code":86101,"message":"waiting scan"}}`), nil
			}
			if pollCount == 2 {
				return bilibiliJSONResponse(`{"code":0,"data":{"code":86090,"message":"waiting confirm"}}`), nil
			}
			return bilibiliJSONResponse(`{
				"code": 0,
				"data": {
					"code": 0,
					"url": "https://passport.bilibili.com/login?SESSDATA=fixture&bili_jct=fixture&DedeUserID=123456",
					"refresh_token": "fixture-refresh"
				}
			}`), nil
		case navURL:
			return bilibiliJSONResponse(`{
				"code": 0,
				"data": {
					"isLogin": true,
					"mid": "123456",
					"uname": "测试账号昵称",
					"face": "https://i0.hdslb.com/bfs/face/test-account.jpg"
				}
			}`), nil
		default:
			return nil, fmt.Errorf("unexpected request url: %s", request.URL.String())
		}
	}), func() time.Time { return time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC) })

	created, err := service.Create(context.Background())
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.State != QRLoginPendingScan || created.QRCodeURL == "" || created.LoginID == "" || created.ExpiresAt.Format(time.RFC3339) != "2026-06-08T08:03:00Z" {
		t.Fatalf("unexpected qr create result: %#v", created)
	}

	scan, err := service.Poll(context.Background(), created.LoginID)
	if err != nil {
		t.Fatalf("Poll scan returned error: %v", err)
	}
	if scan.State != QRLoginPendingScan || scan.Cookie != "" {
		t.Fatalf("unexpected qr scan result: %#v", scan)
	}

	confirm, err := service.Poll(context.Background(), created.LoginID)
	if err != nil {
		t.Fatalf("Poll confirm returned error: %v", err)
	}
	if confirm.State != QRLoginPendingConfirm || confirm.Cookie != "" {
		t.Fatalf("unexpected qr confirm result: %#v", confirm)
	}

	succeeded, err := service.Poll(context.Background(), created.LoginID)
	if err != nil {
		t.Fatalf("Poll success returned error: %v", err)
	}
	if succeeded.State != QRLoginSucceeded {
		t.Fatalf("unexpected qr success state: %#v", succeeded)
	}
	for _, fragment := range []string{"SESSDATA=fixture", "bili_jct=fixture", "DedeUserID=123456", "ac_time_value=fixture-refresh"} {
		if !strings.Contains(succeeded.Cookie, fragment) {
			t.Fatalf("success cookie missing %s: %s", fragment, succeeded.Cookie)
		}
	}
	if succeeded.Account.UID != "123456" || succeeded.Account.Nickname != "测试账号昵称" || succeeded.Account.AvatarURL != "https://i0.hdslb.com/bfs/face/test-account.jpg" {
		t.Fatalf("unexpected qr account: %#v", succeeded.Account)
	}
}

func TestQRLoginServicePollExpiredSession(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	service := NewQRLoginService(bilibiliRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return bilibiliJSONResponse(`{"code":0,"data":{"url":"https://passport.bilibili.com/scan?qrcode_key=fixture-key","qrcode_key":"fixture-key"}}`), nil
	}), func() time.Time { return now })

	created, err := service.Create(context.Background())
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	now = now.Add(4 * time.Minute)
	expired, err := service.Poll(context.Background(), created.LoginID)
	if err != nil {
		t.Fatalf("Poll expired returned error: %v", err)
	}
	if expired.State != QRLoginExpired || expired.Cookie != "" {
		t.Fatalf("unexpected expired result: %#v", expired)
	}
}

type bilibiliRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn bilibiliRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func bilibiliJSONResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
