package bilibili

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSessionClientSignURLAddsWBIParameters(t *testing.T) {
	t.Parallel()

	client := NewSessionClient(bilibiliRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != biliTicketURL+"?context%5Bts%5D=1780905600&csrf=csrf&hexsign=4d01c920483d0aa10c9c016630baa943658e323df7233782f4971368c58cb634&key_id=ec02" {
			t.Fatalf("unexpected ticket url: %s", request.URL.String())
		}
		return bilibiliJSONResponse(`{
			"code": 0,
			"data": {
				"ticket": "ticket-value",
				"created_at": 1780905600,
				"ttl": 259200,
				"nav": {
					"img": "https://i0.hdslb.com/bfs/wbi/7cd084941338484aae1ad9425b84077c.png",
					"sub": "https://i0.hdslb.com/bfs/wbi/4932caff0ff746eab6f01bf08b70ac45.png"
				}
			}
		}`), nil
	}), func() time.Time { return time.Unix(1780905600, 0).UTC() })

	signed, err := client.SignURL(context.Background(), "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/all?type=all&page=1", "SESSDATA=fixture; bili_jct=csrf;")
	if err != nil {
		t.Fatalf("SignURL returned error: %v", err)
	}
	if !strings.Contains(signed, "wts=1780905600") {
		t.Fatalf("signed url missing wts: %s", signed)
	}
	if !strings.Contains(signed, "w_rid=389c1304f65697bbde60fdd8f8a6f9b6") {
		t.Fatalf("signed url missing expected w_rid: %s", signed)
	}
	if strings.Contains(signed, "!") || strings.Contains(signed, "'") {
		t.Fatalf("signed url contains unsanitized WBI characters: %s", signed)
	}
}

func TestPrepareCookieRefreshesAndEnrichesCookie(t *testing.T) {
	t.Parallel()

	client := NewSessionClient(bilibiliRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case strings.HasPrefix(request.URL.String(), cookieInfoURL):
			return bilibiliJSONResponse(`{"code":0,"data":{"refresh":true,"timestamp":1780905600000}}`), nil
		case strings.HasPrefix(request.URL.String(), correspondBaseURL):
			return bilibiliJSONResponse(`<html><div id="1-name">refresh-csrf</div></html>`), nil
		case request.URL.String() == cookieRefreshURL:
			response := bilibiliJSONResponse(`{"code":0,"data":{"status":0,"refresh_token":"new-refresh"}}`)
			response.Header.Set("Set-Cookie", "SESSDATA=new-sess; Path=/; Domain=.bilibili.com")
			response.Header.Add("Set-Cookie", "bili_jct=new-csrf; Path=/; Domain=.bilibili.com")
			return response, nil
		case request.URL.String() == cookieRefreshConfirmURL:
			return bilibiliJSONResponse(`{"code":0}`), nil
		case request.URL.String() == buvidSPIURL:
			return bilibiliJSONResponse(`{"code":0,"data":{"b_3":"buvid3-value","b_4":"buvid4-value"}}`), nil
		case strings.HasPrefix(request.URL.String(), biliTicketURL):
			return bilibiliJSONResponse(`{"code":0,"data":{"ticket":"ticket-value","created_at":1780905600,"ttl":259200}}`), nil
		default:
			t.Fatalf("unexpected request url: %s", request.URL.String())
			return nil, nil
		}
	}), func() time.Time { return time.Unix(1780905600, 0).UTC() })

	prepared, err := client.PrepareCookie(context.Background(), "SESSDATA=old-sess; bili_jct=old-csrf; DedeUserID=123456; ac_time_value=old-refresh;")
	if err != nil {
		t.Fatalf("PrepareCookie returned error: %v", err)
	}
	if !prepared.Refreshed || !prepared.Enriched {
		t.Fatalf("expected refreshed and enriched cookie: %#v", prepared)
	}
	for _, want := range []string{
		"SESSDATA=new-sess",
		"bili_jct=new-csrf",
		"DedeUserID=123456",
		"ac_time_value=new-refresh",
		"buvid3=buvid3-value",
		"buvid4=buvid4-value",
		"b_nut=1780905600",
		"bili_ticket=ticket-value",
		"bili_ticket_expires=1781164800",
	} {
		if !strings.Contains(prepared.Cookie, want) {
			t.Fatalf("prepared cookie missing %s: %s", want, prepared.Cookie)
		}
	}
}

func TestValidateCookieForLoginRequiresSESSDATA(t *testing.T) {
	t.Parallel()

	err := validateCookieForLogin("bili_jct=csrf;")
	if err == nil {
		t.Fatalf("expected missing SESSDATA error")
	}
	biliErr := asBilibiliError(err)
	if biliErr == nil || biliErr.Kind != ErrorAuth {
		t.Fatalf("unexpected error: %#v", err)
	}
}

func TestPrepareCookieKeepsCookieWhenRefreshCheckRiskFails(t *testing.T) {
	t.Parallel()

	client := NewSessionClient(bilibiliRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case strings.HasPrefix(request.URL.String(), cookieInfoURL):
			return &http.Response{
				StatusCode: http.StatusPreconditionFailed,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"code":-412,"message":"请求被拦截"}`)),
				Request:    request,
			}, nil
		case request.URL.String() == buvidSPIURL:
			return bilibiliJSONResponse(`{"code":0,"data":{"b_3":"buvid3-value","b_4":"buvid4-value"}}`), nil
		case strings.HasPrefix(request.URL.String(), biliTicketURL):
			return bilibiliJSONResponse(`{"code":0,"data":{"ticket":"ticket-value","created_at":1780905600,"ttl":259200}}`), nil
		default:
			t.Fatalf("unexpected request url: %s", request.URL.String())
			return nil, nil
		}
	}), func() time.Time { return time.Unix(1780905600, 0).UTC() })

	prepared, err := client.PrepareCookie(context.Background(), "SESSDATA=old-sess; bili_jct=old-csrf; DedeUserID=123456; ac_time_value=old-refresh;")
	if err != nil {
		t.Fatalf("PrepareCookie should keep usable cookie on refresh-check risk error: %v", err)
	}
	if prepared.Refreshed || !prepared.Enriched || !strings.Contains(prepared.Cookie, "SESSDATA=old-sess") || !strings.Contains(prepared.Cookie, "buvid3=buvid3-value") {
		t.Fatalf("unexpected prepared cookie: %#v", prepared)
	}
}

func TestClassifyBilibiliRiskAndAuthErrors(t *testing.T) {
	t.Parallel()

	risk := apiError(http.StatusOK, -352, "风控校验失败", []byte(`{"code":-352}`))
	if biliErr := asBilibiliError(risk); biliErr == nil || biliErr.Kind != ErrorRiskControl || !shouldRetryWBI(risk) {
		t.Fatalf("unexpected risk error: %#v", risk)
	}
	auth := apiError(http.StatusOK, -101, "账号未登录", []byte(`{"code":-101}`))
	if biliErr := asBilibiliError(auth); biliErr == nil || biliErr.Kind != ErrorAuth || !isBilibiliAuthError(auth) {
		t.Fatalf("unexpected auth error: %#v", auth)
	}
	csrf := apiError(http.StatusOK, -111, "csrf 校验失败", []byte(`{"code":-111}`))
	if biliErr := asBilibiliError(csrf); biliErr == nil || biliErr.Kind != ErrorCSRF {
		t.Fatalf("unexpected csrf error: %#v", csrf)
	}
	rateLimit := apiError(http.StatusOK, -509, "请求过于频繁", []byte(`{"code":-509}`))
	if biliErr := asBilibiliError(rateLimit); biliErr == nil || biliErr.Kind != ErrorRateLimit {
		t.Fatalf("unexpected rate limit error: %#v", rateLimit)
	}
	serverErr := apiError(http.StatusInternalServerError, 0, "", []byte(`{"code":0}`))
	if biliErr := asBilibiliError(serverErr); biliErr == nil || biliErr.Kind != ErrorServer {
		t.Fatalf("unexpected server error: %#v", serverErr)
	}
}
