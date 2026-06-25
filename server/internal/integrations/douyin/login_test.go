package douyin

import (
	"context"
	"errors"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"strings"
	"testing"
	"time"
)

func TestCreateUsesBrowserQRCodeResult(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 8, 0, 0, 0, time.UTC)
	browser := &stubDouyinBrowser{createResult: BrowserCreateResult{
		Token:     "fixture-douyin-token",
		QRCodeURL: "https://api.amemv.com/ucenter_web/app/aweme/scan_login/index/douyin_scan_code_login/cn/app/index.html?token=fixture-douyin-token",
		ExpiresAt: now.Add(3 * time.Minute),
		Cookies:   map[string]string{"ttwid": "fixture-ttwid"},
	}}
	provider := NewProvider(nil, browser)

	session, err := provider.Create(context.Background(), now)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if browser.createCalls != 1 {
		t.Fatalf("browser create calls = %d, want 1", browser.createCalls)
	}
	if session.Platform != thirdparty.PlatformDouyin || session.Token != "fixture-douyin-token" || session.QRCodeURL == "" {
		t.Fatalf("unexpected session: %#v", session)
	}
	if session.Values["mode"] == douyinHTTPMode {
		t.Fatalf("browser session should not be marked as HTTP mode: %#v", session.Values)
	}
	if session.Cookies["ttwid"] != "fixture-ttwid" {
		t.Fatalf("session cookies = %#v, want ttwid", session.Cookies)
	}
}

func TestCreateReturnsBrowserErrorWithoutHTTPFallback(t *testing.T) {
	t.Parallel()

	browser := &stubDouyinBrowser{createErr: errors.New("context canceled")}
	provider := NewProvider(thirdparty.NewHTTPClient(nil), browser)

	_, err := provider.Create(context.Background(), time.Date(2026, 6, 22, 8, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("Create error = nil, want browser error")
	}
	if !strings.Contains(err.Error(), "douyin browser login failed") {
		t.Fatalf("Create error = %q, want browser failure", err.Error())
	}
	if strings.Contains(err.Error(), "http fallback") {
		t.Fatalf("Create error should not include HTTP fallback: %q", err.Error())
	}
}

func TestParseDouyinBrowserQRCodeResponse(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 8, 0, 0, 0, time.UTC)
	result, err := parseDouyinBrowserQRCodeResponse([]byte(`{"data":{"error_code":0,"token":"fixture-token","qrcode_index_url":"https://api.amemv.com/scan?token=fixture-token","expire_time":1782063677},"message":"success"}`), now)
	if err != nil {
		t.Fatalf("parseDouyinBrowserQRCodeResponse: %v", err)
	}
	if result.Token != "fixture-token" || result.QRCodeURL != "https://api.amemv.com/scan?token=fixture-token" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if !result.ExpiresAt.After(now) {
		t.Fatalf("ExpiresAt = %s, want after %s", result.ExpiresAt, now)
	}
}

func TestParseDouyinBrowserQRCodeResponseRejectsHTML(t *testing.T) {
	t.Parallel()

	_, err := parseDouyinBrowserQRCodeResponse([]byte(`<!doctype html><html></html>`), time.Date(2026, 6, 22, 8, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("parseDouyinBrowserQRCodeResponse error = nil, want JSON error")
	}
}

func TestParseDouyinBrowserPollState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{name: "new", body: `{"data":{"error_code":0,"status":"new"},"message":"success"}`, want: "pending_scan"},
		{name: "scanned", body: `{"data":{"error_code":0,"status":"scanned"},"message":"success"}`, want: "pending_confirm"},
		{name: "success", body: `{"data":{"error_code":0,"status":"success"},"message":"success"}`, want: "succeeded"},
		{name: "numeric success", body: `{"data":{"error_code":0,"status":3},"message":"success"}`, want: "succeeded"},
		{name: "expired", body: `{"data":{"error_code":0,"status":"expired"},"message":"success"}`, want: "expired"},
		{name: "risk blocked", body: `{"data":{"error_code":1105,"description":"您正在尝试访问的网站存在安全风险，为保护您的抖音账号安全与隐私，已阻止此次访问。"},"message":"error"}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseDouyinBrowserPollState([]byte(tt.body))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseDouyinBrowserPollState error = nil, want error")
				}
				if !errors.Is(err, errDouyinQRCodePollBlocked) {
					t.Fatalf("parseDouyinBrowserPollState error = %v, want %v", err, errDouyinQRCodePollBlocked)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDouyinBrowserPollState: %v", err)
			}
			if got != tt.want {
				t.Fatalf("state = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsDouyinQRCodePollBlocked(t *testing.T) {
	t.Parallel()

	tests := []struct {
		message string
		blocked bool
	}{
		{"您正在尝试访问的网站存在安全风险，为保护您的抖音账号安全与隐私，已阻止此次访问。", true},
		{"安全风险", true},
		{"已阻止此次访问", true},
		{"正常响应", false},
		{"success", false},
	}
	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			t.Parallel()
			if got := isDouyinQRCodePollBlocked(tt.message); got != tt.blocked {
				t.Fatalf("isDouyinQRCodePollBlocked(%q) = %v, want %v", tt.message, got, tt.blocked)
			}
		})
	}
}

func TestDouyinBrowserCreateTimeoutFitsWebRequestTimeout(t *testing.T) {
	t.Parallel()

	if douyinBrowserCreateTimeout >= 30*time.Second {
		t.Fatalf("douyin browser create timeout = %s, want less than web request timeout", douyinBrowserCreateTimeout)
	}
}

type stubDouyinBrowser struct {
	createResult BrowserCreateResult
	createErr    error
	pollResult   BrowserPollResult
	pollErr      error
	createCalls  int
	pollCalls    int
	closeTokens  []string
}

func (s *stubDouyinBrowser) Create(context.Context, time.Time) (BrowserCreateResult, error) {
	s.createCalls++
	return s.createResult, s.createErr
}

func (s *stubDouyinBrowser) Poll(context.Context, string) (BrowserPollResult, error) {
	s.pollCalls++
	return s.pollResult, s.pollErr
}

func (s *stubDouyinBrowser) Close(token string) {
	s.closeTokens = append(s.closeTokens, token)
}
