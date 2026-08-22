package thirdparty

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestFetchAccountAvatarSupportsCurrentPlatformResponses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		platform        string
		url             string
		referer         string
		upstreamType    string
		wantContentType string
	}{
		{
			name:            "bilibili jpeg",
			platform:        PlatformBilibili,
			url:             "https://i1.hdslb.com/bfs/face/avatar.jpg",
			referer:         "https://www.bilibili.com/",
			upstreamType:    "image/jpeg",
			wantContentType: "image/jpeg",
		},
		{
			name:            "weibo signed jpeg",
			platform:        PlatformWeibo,
			url:             "https://wx1.sinaimg.cn/orj480/avatar.jpg?KID=imgbed,tva#preview",
			referer:         "https://weibo.com/",
			upstreamType:    "image/jpeg; charset=binary",
			wantContentType: "image/jpeg",
		},
		{
			name:            "douyin signed webp",
			platform:        PlatformDouyin,
			url:             "https://p3-pc-sign.douyinpic.com/aweme/720x720/avatar.jpeg?x-expires=fixture&x-signature=fixture",
			referer:         "https://www.douyin.com/",
			upstreamType:    "image/webp",
			wantContentType: "image/webp",
		},
		{
			name:            "netease jpg alias",
			platform:        PlatformNeteaseMusic,
			url:             "https://p4.music.126.net/fixture==/avatar.jpg",
			referer:         "https://music.163.com/",
			upstreamType:    "image/jpg",
			wantContentType: "image/jpeg",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				wantURL := strings.TrimSuffix(test.url, "#preview")
				if got := request.URL.String(); got != wantURL {
					t.Fatalf("avatar URL = %q, want %q", got, wantURL)
				}
				if got := request.Header.Get("Referer"); got != test.referer {
					t.Fatalf("Referer = %q, want %q", got, test.referer)
				}
				if request.Header.Get("Cookie") != "" {
					t.Fatal("avatar request must not forward account credentials")
				}
				return &http.Response{
					StatusCode:    http.StatusOK,
					Header:        http.Header{"Content-Type": []string{test.upstreamType}},
					Body:          io.NopCloser(strings.NewReader("image")),
					ContentLength: 5,
					Request:       request,
				}, nil
			})}

			resource, err := FetchAccountAvatar(context.Background(), client, test.platform, test.url)
			if err != nil {
				t.Fatalf("FetchAccountAvatar returned error: %v", err)
			}
			if resource.ContentType != test.wantContentType || string(resource.Body) != "image" {
				t.Fatalf("unexpected avatar resource: %#v", resource)
			}
		})
	}
}

func TestNormalizeAccountAvatarURLAllowsOnlyPlatformAvatarHosts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		platform string
		rawURL   string
		wantURL  string
		referer  string
	}{
		{
			name:     "bilibili",
			platform: PlatformBilibili,
			rawURL:   "https://i0.hdslb.com/bfs/face/avatar.jpg",
			wantURL:  "https://i0.hdslb.com/bfs/face/avatar.jpg",
			referer:  "https://www.bilibili.com/",
		},
		{
			name:     "weibo signed",
			platform: PlatformWeibo,
			rawURL:   "https://tvax1.sinaimg.cn/crop/avatar.jpg?KID=imgbed,tva",
			wantURL:  "https://tvax1.sinaimg.cn/crop/avatar.jpg?KID=imgbed,tva",
			referer:  "https://weibo.com/",
		},
		{
			name:     "douyin",
			platform: PlatformDouyin,
			rawURL:   "https://p3-pc.douyinpic.com/avatar.jpg",
			wantURL:  "https://p3-pc.douyinpic.com/avatar.jpg",
			referer:  "https://www.douyin.com/",
		},
		{
			name:     "netease music",
			platform: PlatformNeteaseMusic,
			rawURL:   "https://p1.music.126.net/avatar.jpg",
			wantURL:  "https://p1.music.126.net/avatar.jpg",
			referer:  "https://music.163.com/",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			gotURL, gotReferer, err := normalizeAccountAvatarURL(test.platform, test.rawURL)
			if err != nil {
				t.Fatalf("normalizeAccountAvatarURL returned error: %v", err)
			}
			if gotURL != test.wantURL || gotReferer != test.referer {
				t.Fatalf("normalizeAccountAvatarURL = (%q, %q), want (%q, %q)", gotURL, gotReferer, test.wantURL, test.referer)
			}
		})
	}
}

func TestNormalizeAccountAvatarURLRejectsUnsafeSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		platform string
		rawURL   string
	}{
		{name: "plain HTTP", platform: PlatformWeibo, rawURL: "http://wx1.sinaimg.cn/avatar.jpg"},
		{name: "credentials", platform: PlatformWeibo, rawURL: "https://user@wx1.sinaimg.cn/avatar.jpg"},
		{name: "nonstandard port", platform: PlatformWeibo, rawURL: "https://wx1.sinaimg.cn:8443/avatar.jpg"},
		{name: "lookalike host", platform: PlatformWeibo, rawURL: "https://evilsinaimg.cn/avatar.jpg"},
		{name: "platform mismatch", platform: PlatformWeibo, rawURL: "https://i0.hdslb.com/bfs/face/avatar.jpg"},
		{name: "root path", platform: PlatformWeibo, rawURL: "https://wx1.sinaimg.cn/"},
		{name: "unsupported platform", platform: "unknown", rawURL: "https://wx1.sinaimg.cn/avatar.jpg"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, _, err := normalizeAccountAvatarURL(test.platform, test.rawURL); !errors.Is(err, ErrAccountAvatarURLUnsupported) {
				t.Fatalf("normalizeAccountAvatarURL error = %v, want ErrAccountAvatarURLUnsupported", err)
			}
		})
	}
}

func TestFetchAccountAvatarRejectsNonImageAndOversizedResponses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		response  *http.Response
		wantError error
	}{
		{
			name: "non image",
			response: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/html"}},
				Body:       io.NopCloser(strings.NewReader("not an image")),
			},
			wantError: ErrAccountAvatarContentTypeUnsupported,
		},
		{
			name: "oversized",
			response: &http.Response{
				StatusCode:    http.StatusOK,
				Header:        http.Header{"Content-Type": []string{"image/jpeg"}},
				Body:          io.NopCloser(strings.NewReader("ignored")),
				ContentLength: maxAccountAvatarBytes + 1,
			},
			wantError: ErrAccountAvatarReadFailed,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				response := *test.response
				response.Request = request
				return &response, nil
			})}
			_, err := FetchAccountAvatar(context.Background(), client, PlatformWeibo, "https://wx1.sinaimg.cn/avatar.jpg")
			if !errors.Is(err, test.wantError) {
				t.Fatalf("FetchAccountAvatar error = %v, want %v", err, test.wantError)
			}
		})
	}
}
