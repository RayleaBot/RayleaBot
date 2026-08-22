package weibo

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

type profileRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn profileRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestGetWeiboJSONRejectsRedirectOutsideThirdPartyAllowlist(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	client := thirdparty.NewHTTPClient(profileRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests.Add(1)
		if request.URL.Hostname() != "m.weibo.cn" {
			t.Fatalf("redirect reached disallowed host %q", request.URL.Hostname())
		}
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{"https://127.0.0.1/private"}},
			Body:       io.NopCloser(strings.NewReader("redirect")),
			Request:    request,
		}, nil
	}))

	var target map[string]any
	err := getWeiboJSON(
		context.Background(),
		client,
		weiboMobileConfigURL,
		weiboProfileHeaders("https://m.weibo.cn/"),
		map[string]string{"SUB": "fixture"},
		&target,
	)
	if err == nil {
		t.Fatal("redirect outside allowlist was accepted")
	}
	if requests.Load() != 1 {
		t.Fatalf("transport requests = %d, want 1", requests.Load())
	}
}

func TestFetchWeiboAvatarFromMobilePageUsesConfiguredTransport(t *testing.T) {
	t.Parallel()

	const avatarURL = "https://wx1.sinaimg.cn/avatar.jpg"
	client := thirdparty.NewHTTPClient(profileRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://m.weibo.cn/u/123456" {
			t.Fatalf("profile page URL = %q", request.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
			Body:       io.NopCloser(strings.NewReader(`<meta property="og:image" content="` + avatarURL + `">`)),
			Request:    request,
		}, nil
	}))

	if got := fetchWeiboAvatarFromMobilePage(context.Background(), client, "123456", map[string]string{"SUB": "fixture"}); got != avatarURL {
		t.Fatalf("avatar URL = %q, want %q", got, avatarURL)
	}
}
