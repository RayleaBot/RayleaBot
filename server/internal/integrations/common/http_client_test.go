package common

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestValidateThirdPartyURL(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		wantErr bool
	}{
		{name: "allowed bilibili api", rawURL: "https://api.bilibili.com/x/web-interface/nav"},
		{name: "allowed douyin short link", rawURL: "https://v.douyin.com/fixture/"},
		{name: "allowed netease music", rawURL: "https://music.163.com/api/song/detail"},
		{name: "allowed weibo login", rawURL: "https://login.sina.com.cn/sso/login.php"},
		{name: "reject http", rawURL: "http://api.bilibili.com/x/web-interface/nav", wantErr: true},
		{name: "reject userinfo", rawURL: "https://api.bilibili.com@127.0.0.1/secret", wantErr: true},
		{name: "reject loopback", rawURL: "https://127.0.0.1/secret", wantErr: true},
		{name: "reject unrelated 163 host", rawURL: "https://example.163.com/secret", wantErr: true},
		{name: "reject malicious suffix", rawURL: "https://api.bilibili.com.evil.test/x", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateThirdPartyURL(tt.rawURL)
			if tt.wantErr && err == nil {
				t.Fatalf("ValidateThirdPartyURL(%q) succeeded, want error", tt.rawURL)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ValidateThirdPartyURL(%q) returned error: %v", tt.rawURL, err)
			}
		})
	}
}

func TestHostMatchesRequiresHostnameBoundary(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		suffixes []string
		want     bool
	}{
		{name: "exact", host: "music.163.com", suffixes: []string{"music.163.com"}, want: true},
		{name: "subdomain", host: "api.bilibili.com", suffixes: []string{"bilibili.com"}, want: true},
		{name: "case and trailing dot", host: "M.Weibo.CN.", suffixes: []string{".weibo.cn"}, want: true},
		{name: "prefix confusion", host: "evilmusic.163.com", suffixes: []string{"music.163.com"}, want: false},
		{name: "suffix confusion", host: "api.bilibili.com.evil.test", suffixes: []string{"bilibili.com"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HostMatches(tt.host, tt.suffixes...); got != tt.want {
				t.Fatalf("HostMatches(%q, %q) = %v, want %v", tt.host, tt.suffixes, got, tt.want)
			}
		})
	}
}

func TestFollowGetRejectsUnsafeRedirect(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": {"https://127.0.0.1/secret"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    request,
		}, nil
	})}

	err := FollowGet(context.Background(), client, "https://www.douyin.com/", nil, map[string]string{})
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("FollowGet error = %v, want unsafe redirect rejection", err)
	}
	if calls != 1 {
		t.Fatalf("transport calls = %d, want only initial allowed request", calls)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
