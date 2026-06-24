package credential

import (
	"context"
	"errors"
	"strings"
	"testing"

	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/httpaction"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

func TestInjectorUsesConfiguredAccount(t *testing.T) {
	t.Parallel()

	account := thirdparty.Account{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}
	accounts := &stubAccounts{
		accounts: []thirdparty.Account{account},
		cookies:  map[string]string{"primary": "SESSDATA=fixture; bili_jct=csrf;"},
	}
	headers := map[string]string{"Accept": "application/json"}

	result, err := NewInjector(accounts, nil).Inject(context.Background(), httpaction.CredentialRequest{
		PluginID:   SubscriptionHubPluginID,
		RawURL:     "https://api.bilibili.com/x/web-interface/nav",
		ScopeHosts: []string{"api.bilibili.com"},
		Headers:    headers,
	})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}
	if got := headers["Cookie"]; got != "SESSDATA=fixture; bili_jct=csrf;" {
		t.Fatalf("unexpected Cookie header: %q", got)
	}
	if len(accounts.marked) != 0 {
		t.Fatalf("injector should not mark usage before the HTTP request succeeds")
	}
	if result.AfterSuccess == nil {
		t.Fatalf("expected success callback")
	}
	if err := result.AfterSuccess(context.Background()); err != nil {
		t.Fatalf("AfterSuccess failed: %v", err)
	}
	if len(accounts.marked) != 1 || accounts.marked[0] != "primary" {
		t.Fatalf("unexpected marked accounts: %#v", accounts.marked)
	}
}

func TestInjectorPreparesAndStoresUpdatedCookie(t *testing.T) {
	t.Parallel()

	account := thirdparty.Account{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}
	accounts := &stubAccounts{
		accounts: []thirdparty.Account{account},
		cookies:  map[string]string{"primary": "SESSDATA=old; bili_jct=csrf;"},
	}
	session := &stubSession{
		prepared: bilibilisession.PreparedCookie{
			Cookie:   "SESSDATA=new; bili_jct=csrf; buvid3=device;",
			Enriched: true,
		},
	}
	headers := map[string]string{}

	_, err := NewInjector(accounts, session).Inject(context.Background(), httpaction.CredentialRequest{
		PluginID:   SubscriptionHubPluginID,
		RawURL:     "https://api.bilibili.com/x/web-interface/nav",
		ScopeHosts: []string{"api.bilibili.com"},
		Headers:    headers,
	})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}
	if got := headers["Cookie"]; got != "SESSDATA=new; bili_jct=csrf; buvid3=device;" {
		t.Fatalf("unexpected prepared cookie header: %q", got)
	}
	if got := accounts.cookies["primary"]; got != "SESSDATA=new; bili_jct=csrf; buvid3=device;" {
		t.Fatalf("updated cookie was not stored: %q", got)
	}
}

func TestInjectorSignsWBIURL(t *testing.T) {
	t.Parallel()

	account := thirdparty.Account{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}
	accounts := &stubAccounts{
		accounts: []thirdparty.Account{account},
		cookies:  map[string]string{"primary": "SESSDATA=fixture; bili_jct=csrf;"},
	}
	session := &stubSession{
		prepared: bilibilisession.PreparedCookie{Cookie: "SESSDATA=fixture; bili_jct=csrf;"},
		signed:   "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/all?w_rid=signed",
	}
	headers := map[string]string{}

	result, err := NewInjector(accounts, session).Inject(context.Background(), httpaction.CredentialRequest{
		PluginID:   SubscriptionHubPluginID,
		RawURL:     "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/all",
		ScopeHosts: []string{"api.bilibili.com"},
		Headers:    headers,
	})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}
	if result.URL != "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/all?w_rid=signed" {
		t.Fatalf("unexpected signed URL: %q", result.URL)
	}
	if session.signCookie != "SESSDATA=fixture; bili_jct=csrf;" {
		t.Fatalf("unexpected sign cookie: %q", session.signCookie)
	}
}

func TestInjectorReturnsBilibiliSignError(t *testing.T) {
	t.Parallel()

	account := thirdparty.Account{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}
	accounts := &stubAccounts{
		accounts: []thirdparty.Account{account},
		cookies:  map[string]string{"primary": "SESSDATA=fixture; bili_jct=csrf;"},
	}
	session := &stubSession{
		prepared: bilibilisession.PreparedCookie{Cookie: "SESSDATA=fixture; bili_jct=csrf;"},
		signErr:  errors.New("sign failed"),
	}

	_, err := NewInjector(accounts, session).Inject(context.Background(), httpaction.CredentialRequest{
		PluginID:   SubscriptionHubPluginID,
		RawURL:     "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/all",
		ScopeHosts: []string{"api.bilibili.com"},
		Headers:    map[string]string{},
	})
	if err == nil || !strings.Contains(err.Error(), "sign failed") {
		t.Fatalf("expected sign error, got %v", err)
	}
}

func TestInjectorSkipsNonBilibiliHosts(t *testing.T) {
	t.Parallel()

	accounts := &stubAccounts{
		accounts: []thirdparty.Account{{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}},
		cookies:  map[string]string{"primary": "SESSDATA=fixture;"},
	}
	headers := map[string]string{}

	result, err := NewInjector(accounts, nil).Inject(context.Background(), httpaction.CredentialRequest{
		PluginID:   SubscriptionHubPluginID,
		RawURL:     "https://example.com/api",
		ScopeHosts: []string{"example.com"},
		Headers:    headers,
	})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}
	if result.AfterSuccess != nil {
		t.Fatalf("unexpected success callback for non-Bilibili host")
	}
	if _, ok := headers["Cookie"]; ok {
		t.Fatalf("unexpected Cookie header: %#v", headers)
	}
}

func TestInjectorUsesConfiguredWeiboAccount(t *testing.T) {
	t.Parallel()

	account := thirdparty.Account{Platform: thirdparty.PlatformWeibo, AccountID: "primary"}
	accounts := &stubAccounts{
		accounts: []thirdparty.Account{account},
		cookies:  map[string]string{"primary": "SUB=fixture;"},
	}
	headers := map[string]string{}

	result, err := NewInjector(accounts, &stubSession{signErr: errors.New("unexpected sign")}).Inject(context.Background(), httpaction.CredentialRequest{
		PluginID:   SubscriptionHubPluginID,
		RawURL:     "https://m.weibo.cn/statuses/show?id=123456",
		ScopeHosts: []string{"m.weibo.cn"},
		Headers:    headers,
	})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}
	if got := headers["Cookie"]; got != "SUB=fixture;" {
		t.Fatalf("unexpected Cookie header: %q", got)
	}
	if result.URL != "" {
		t.Fatalf("unexpected signed URL for Weibo: %q", result.URL)
	}
	if result.AfterSuccess == nil {
		t.Fatalf("expected success callback")
	}
	if err := result.AfterSuccess(context.Background()); err != nil {
		t.Fatalf("AfterSuccess failed: %v", err)
	}
	if len(accounts.marked) != 1 || accounts.marked[0] != "primary" {
		t.Fatalf("unexpected marked accounts: %#v", accounts.marked)
	}
}

func TestInjectorDoesNotUseBilibiliCookieForWeibo(t *testing.T) {
	t.Parallel()

	accounts := &stubAccounts{
		accounts: []thirdparty.Account{{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}},
		cookies:  map[string]string{"primary": "SESSDATA=fixture;"},
	}
	headers := map[string]string{}

	result, err := NewInjector(accounts, nil).Inject(context.Background(), httpaction.CredentialRequest{
		PluginID:   SubscriptionHubPluginID,
		RawURL:     "https://m.weibo.cn/statuses/show?id=123456",
		ScopeHosts: []string{"m.weibo.cn"},
		Headers:    headers,
	})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}
	if result.AfterSuccess != nil {
		t.Fatalf("unexpected success callback")
	}
	if _, ok := headers["Cookie"]; ok {
		t.Fatalf("unexpected Cookie header: %#v", headers)
	}
}

func TestInjectorKeepsPluginCookie(t *testing.T) {
	t.Parallel()

	accounts := &stubAccounts{
		accounts: []thirdparty.Account{{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}},
		cookies:  map[string]string{"primary": "SESSDATA=fixture;"},
	}
	headers := map[string]string{"cookie": "SESSDATA=plugin;"}

	result, err := NewInjector(accounts, nil).Inject(context.Background(), httpaction.CredentialRequest{
		PluginID:   SubscriptionHubPluginID,
		RawURL:     "https://api.live.bilibili.com/room/v1/Room/get_info",
		ScopeHosts: []string{"api.live.bilibili.com"},
		Headers:    headers,
	})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}
	if result.AfterSuccess != nil {
		t.Fatalf("unexpected success callback when plugin already provided Cookie")
	}
	if got := headers["cookie"]; got != "SESSDATA=plugin;" {
		t.Fatalf("plugin Cookie header was changed: %q", got)
	}
}

func TestInjectorRequiresDeclaredHost(t *testing.T) {
	t.Parallel()

	accounts := &stubAccounts{
		accounts: []thirdparty.Account{{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}},
		cookies:  map[string]string{"primary": "SESSDATA=fixture;"},
	}
	headers := map[string]string{}

	result, err := NewInjector(accounts, nil).Inject(context.Background(), httpaction.CredentialRequest{
		PluginID:   SubscriptionHubPluginID,
		RawURL:     "https://api.bilibili.com/x/web-interface/nav",
		ScopeHosts: []string{"example.com"},
		Headers:    headers,
	})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}
	if result.AfterSuccess != nil {
		t.Fatalf("unexpected success callback without declared host")
	}
	if _, ok := headers["Cookie"]; ok {
		t.Fatalf("unexpected Cookie header: %#v", headers)
	}
}

func TestInjectorRequiresSubscriptionHubPlugin(t *testing.T) {
	t.Parallel()

	accounts := &stubAccounts{
		accounts: []thirdparty.Account{{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}},
		cookies:  map[string]string{"primary": "SESSDATA=fixture;"},
	}
	headers := map[string]string{}

	result, err := NewInjector(accounts, nil).Inject(context.Background(), httpaction.CredentialRequest{
		PluginID:   "raylea.other-plugin",
		RawURL:     "https://api.bilibili.com/x/web-interface/nav",
		ScopeHosts: []string{"api.bilibili.com"},
		Headers:    headers,
	})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}
	if result.AfterSuccess != nil {
		t.Fatalf("unexpected success callback for a non-subscription-hub plugin")
	}
	if _, ok := headers["Cookie"]; ok {
		t.Fatalf("unexpected Cookie header: %#v", headers)
	}
}

func TestIsBilibiliURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		rawURL string
		want   bool
	}{
		{rawURL: "https://bilibili.com", want: true},
		{rawURL: "https://api.bilibili.com/x/web-interface/nav", want: true},
		{rawURL: "https://api.live.bilibili.com/room/v1/Room/get_info", want: true},
		{rawURL: "https://notbilibili.com", want: false},
		{rawURL: "https://bilibili.com.evil.test", want: false},
		{rawURL: "://bad-url", want: false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.rawURL, func(t *testing.T) {
			t.Parallel()
			if got := IsBilibiliURL(tt.rawURL); got != tt.want {
				t.Fatalf("IsBilibiliURL(%q) = %v, want %v", tt.rawURL, got, tt.want)
			}
		})
	}
}

func TestIsBilibiliURLForWBI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		rawURL string
		want   bool
	}{
		{rawURL: "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/all", want: true},
		{rawURL: "https://api.live.bilibili.com/room/v1/Room/get_info", want: true},
		{rawURL: "https://www.bilibili.com/video/BV1RayleaBot", want: false},
		{rawURL: "https://api.bilibili.com.evil.test/x", want: false},
		{rawURL: "://bad-url", want: false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.rawURL, func(t *testing.T) {
			t.Parallel()
			if got := IsBilibiliURLForWBI(tt.rawURL); got != tt.want {
				t.Fatalf("IsBilibiliURLForWBI(%q) = %v, want %v", tt.rawURL, got, tt.want)
			}
		})
	}
}

func TestPlatformForURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		rawURL string
		want   string
	}{
		{rawURL: "https://api.bilibili.com/x/web-interface/nav", want: thirdparty.PlatformBilibili},
		{rawURL: "https://m.weibo.cn/statuses/show?id=123", want: thirdparty.PlatformWeibo},
		{rawURL: "https://v.douyin.com/abc/", want: thirdparty.PlatformDouyin},
		{rawURL: "https://webcast.amemv.com/douyin/webcast/reflow/123", want: thirdparty.PlatformDouyin},
		{rawURL: "https://music.163.com/song?id=123", want: thirdparty.PlatformNeteaseMusic},
		{rawURL: "https://example.com/song?id=123", want: ""},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.rawURL, func(t *testing.T) {
			t.Parallel()
			if got := PlatformForURL(tt.rawURL); got != tt.want {
				t.Fatalf("PlatformForURL(%q) = %q, want %q", tt.rawURL, got, tt.want)
			}
		})
	}
}

type stubAccounts struct {
	accounts []thirdparty.Account
	cookies  map[string]string
	errs     map[string]error
	marked   []string
}

func (s *stubAccounts) ListEnabled(_ context.Context, platform string) ([]thirdparty.Account, error) {
	items := make([]thirdparty.Account, 0, len(s.accounts))
	for _, account := range s.accounts {
		if platform == "" || account.Platform == platform {
			items = append(items, account)
		}
	}
	return items, nil
}

func (s *stubAccounts) ReadCookie(_ context.Context, account thirdparty.Account) (string, error) {
	if s.errs != nil {
		if err := s.errs[account.AccountID]; err != nil {
			return "", err
		}
	}
	if s.cookies == nil {
		return "", secrets.ErrNotFound
	}
	cookie, ok := s.cookies[account.AccountID]
	if !ok {
		return "", secrets.ErrNotFound
	}
	return cookie, nil
}

func (s *stubAccounts) MarkUsed(_ context.Context, account thirdparty.Account) error {
	if account.AccountID == "mark-error" {
		return errors.New("mark used")
	}
	s.marked = append(s.marked, account.AccountID)
	return nil
}

func (s *stubAccounts) UpdateCookie(_ context.Context, account thirdparty.Account, cookie string) error {
	if s.cookies == nil {
		s.cookies = map[string]string{}
	}
	s.cookies[account.AccountID] = cookie
	return nil
}

type stubSession struct {
	prepared   bilibilisession.PreparedCookie
	err        error
	signed     string
	signErr    error
	signCookie string
}

func (s *stubSession) PrepareCookie(context.Context, string) (bilibilisession.PreparedCookie, error) {
	return s.prepared, s.err
}

func (s *stubSession) SignURL(_ context.Context, rawURL, cookie string) (string, error) {
	s.signCookie = cookie
	if s.signErr != nil {
		return rawURL, s.signErr
	}
	if s.signed != "" {
		return s.signed, nil
	}
	return rawURL, nil
}
