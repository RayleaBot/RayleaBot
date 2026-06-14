package actions

import (
	"context"
	"errors"
	"strings"
	"testing"

	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/httpaction"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

func TestApplyBilibiliCookieUsesConfiguredAccount(t *testing.T) {
	t.Parallel()

	account := thirdparty.Account{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}
	accounts := &stubThirdPartyAccounts{
		accounts: []thirdparty.Account{account},
		cookies:  map[string]string{"primary": "SESSDATA=fixture; bili_jct=csrf;"},
	}
	headers := map[string]string{"Accept": "application/json"}

	usedAccount, applied := httpaction.ApplyBilibiliCookie(context.Background(), httpaction.BilibiliCookieRequest{
		PluginID:   httpaction.SubscriptionHubPluginID,
		RawURL:     "https://api.bilibili.com/x/web-interface/nav",
		ScopeHosts: []string{"api.bilibili.com"},
		Headers:    headers,
		ThirdParty: accounts,
	})

	if !applied {
		t.Fatalf("expected Bilibili cookie to be applied")
	}
	if usedAccount.AccountID != "primary" {
		t.Fatalf("unexpected account: %#v", usedAccount)
	}
	if got := headers["Cookie"]; got != "SESSDATA=fixture; bili_jct=csrf;" {
		t.Fatalf("unexpected Cookie header: %q", got)
	}
	if len(accounts.marked) != 0 {
		t.Fatalf("applyBilibiliCookie should not mark usage before the HTTP request succeeds")
	}
}

func TestApplyBilibiliCookiePreparesAndStoresUpdatedCookie(t *testing.T) {
	t.Parallel()

	account := thirdparty.Account{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}
	accounts := &stubThirdPartyAccounts{
		accounts: []thirdparty.Account{account},
		cookies:  map[string]string{"primary": "SESSDATA=old; bili_jct=csrf;"},
	}
	session := &stubBilibiliSession{
		prepared: bilibilisession.PreparedCookie{
			Cookie:   "SESSDATA=new; bili_jct=csrf; buvid3=device;",
			Enriched: true,
		},
	}
	headers := map[string]string{}

	usedAccount, applied := httpaction.ApplyBilibiliCookie(context.Background(), httpaction.BilibiliCookieRequest{
		PluginID:   httpaction.SubscriptionHubPluginID,
		RawURL:     "https://api.bilibili.com/x/web-interface/nav",
		ScopeHosts: []string{"api.bilibili.com"},
		Headers:    headers,
		ThirdParty: accounts,
		Session:    session,
	})

	if !applied || usedAccount.AccountID != "primary" {
		t.Fatalf("expected prepared Bilibili cookie to be applied, got account=%#v applied=%v", usedAccount, applied)
	}
	if got := headers["Cookie"]; got != "SESSDATA=new; bili_jct=csrf; buvid3=device;" {
		t.Fatalf("unexpected prepared cookie header: %q", got)
	}
	if got := accounts.cookies["primary"]; got != "SESSDATA=new; bili_jct=csrf; buvid3=device;" {
		t.Fatalf("updated cookie was not stored: %q", got)
	}
}

func TestExecuteHTTPRequestReturnsBilibiliSignError(t *testing.T) {
	t.Parallel()

	account := thirdparty.Account{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}
	service := &Service{
		grants: &stubGrantView{
			capabilities: map[string]bool{"http.request": true},
			httpHosts:    []string{"api.bilibili.com"},
		},
		thirdParty: &stubThirdPartyAccounts{
			accounts: []thirdparty.Account{account},
			cookies:  map[string]string{"primary": "SESSDATA=fixture; bili_jct=csrf;"},
		},
		bilibiliSession: &stubBilibiliSession{
			prepared: bilibilisession.PreparedCookie{Cookie: "SESSDATA=fixture; bili_jct=csrf;"},
			signErr:  errors.New("sign failed"),
		},
	}

	_, err := service.executeHTTPRequest(context.Background(), httpaction.SubscriptionHubPluginID, runtimeaction.Action{
		HTTPMethod: "GET",
		HTTPURL:    "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/all",
	})

	var runtimeErr *runtimemanager.Error
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("expected runtime error, got %#v", err)
	}
	if runtimeErr.Code != "plugin.internal_error" || !strings.Contains(runtimeErr.Error(), "sign failed") {
		t.Fatalf("unexpected runtime error: %#v", runtimeErr)
	}
}

func TestApplyBilibiliCookieSkipsNonBilibiliHosts(t *testing.T) {
	t.Parallel()

	accounts := &stubThirdPartyAccounts{
		accounts: []thirdparty.Account{{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}},
		cookies:  map[string]string{"primary": "SESSDATA=fixture;"},
	}
	headers := map[string]string{}

	_, applied := httpaction.ApplyBilibiliCookie(context.Background(), httpaction.BilibiliCookieRequest{
		PluginID:   httpaction.SubscriptionHubPluginID,
		RawURL:     "https://example.com/api",
		ScopeHosts: []string{"example.com"},
		Headers:    headers,
		ThirdParty: accounts,
	})

	if applied {
		t.Fatalf("unexpected cookie application for non-Bilibili host")
	}
	if _, ok := headers["Cookie"]; ok {
		t.Fatalf("unexpected Cookie header: %#v", headers)
	}
}

func TestApplyBilibiliCookieKeepsPluginCookie(t *testing.T) {
	t.Parallel()

	accounts := &stubThirdPartyAccounts{
		accounts: []thirdparty.Account{{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}},
		cookies:  map[string]string{"primary": "SESSDATA=fixture;"},
	}
	headers := map[string]string{"cookie": "SESSDATA=plugin;"}

	_, applied := httpaction.ApplyBilibiliCookie(context.Background(), httpaction.BilibiliCookieRequest{
		PluginID:   httpaction.SubscriptionHubPluginID,
		RawURL:     "https://api.live.bilibili.com/room/v1/Room/get_info",
		ScopeHosts: []string{"api.live.bilibili.com"},
		Headers:    headers,
		ThirdParty: accounts,
	})

	if applied {
		t.Fatalf("unexpected cookie application when plugin already provided Cookie")
	}
	if got := headers["cookie"]; got != "SESSDATA=plugin;" {
		t.Fatalf("plugin Cookie header was changed: %q", got)
	}
}

func TestApplyBilibiliCookieRequiresGrantedHost(t *testing.T) {
	t.Parallel()

	accounts := &stubThirdPartyAccounts{
		accounts: []thirdparty.Account{{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}},
		cookies:  map[string]string{"primary": "SESSDATA=fixture;"},
	}
	headers := map[string]string{}

	_, applied := httpaction.ApplyBilibiliCookie(context.Background(), httpaction.BilibiliCookieRequest{
		PluginID:   httpaction.SubscriptionHubPluginID,
		RawURL:     "https://api.bilibili.com/x/web-interface/nav",
		ScopeHosts: []string{"example.com"},
		Headers:    headers,
		ThirdParty: accounts,
	})

	if applied {
		t.Fatalf("unexpected cookie application without granted host")
	}
	if _, ok := headers["Cookie"]; ok {
		t.Fatalf("unexpected Cookie header: %#v", headers)
	}
}

func TestApplyBilibiliCookieRequiresSubscriptionHubPlugin(t *testing.T) {
	t.Parallel()

	accounts := &stubThirdPartyAccounts{
		accounts: []thirdparty.Account{{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}},
		cookies:  map[string]string{"primary": "SESSDATA=fixture;"},
	}
	headers := map[string]string{}

	_, applied := httpaction.ApplyBilibiliCookie(context.Background(), httpaction.BilibiliCookieRequest{
		PluginID:   "raylea.other-plugin",
		RawURL:     "https://api.bilibili.com/x/web-interface/nav",
		ScopeHosts: []string{"api.bilibili.com"},
		Headers:    headers,
		ThirdParty: accounts,
	})

	if applied {
		t.Fatalf("unexpected cookie application for a non-subscription-hub plugin")
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
			if got := httpaction.IsBilibiliURL(tt.rawURL); got != tt.want {
				t.Fatalf("IsBilibiliURL(%q) = %v, want %v", tt.rawURL, got, tt.want)
			}
		})
	}
}

type stubThirdPartyAccounts struct {
	accounts []thirdparty.Account
	cookies  map[string]string
	errs     map[string]error
	marked   []string
}

func (s *stubThirdPartyAccounts) ListEnabled(context.Context, string) ([]thirdparty.Account, error) {
	return append([]thirdparty.Account(nil), s.accounts...), nil
}

func (s *stubThirdPartyAccounts) ReadCookie(_ context.Context, account thirdparty.Account) (string, error) {
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

func (s *stubThirdPartyAccounts) MarkUsed(_ context.Context, account thirdparty.Account) error {
	if account.AccountID == "mark-error" {
		return errors.New("mark used")
	}
	s.marked = append(s.marked, account.AccountID)
	return nil
}

func (s *stubThirdPartyAccounts) UpdateCookie(_ context.Context, account thirdparty.Account, cookie string) error {
	if s.cookies == nil {
		s.cookies = map[string]string{}
	}
	s.cookies[account.AccountID] = cookie
	return nil
}

type stubBilibiliSession struct {
	prepared bilibilisession.PreparedCookie
	err      error
	signed   string
	signErr  error
}

func (s *stubBilibiliSession) PrepareCookie(context.Context, string) (bilibilisession.PreparedCookie, error) {
	return s.prepared, s.err
}

func (s *stubBilibiliSession) SignURL(_ context.Context, rawURL, _ string) (string, error) {
	if s.signErr != nil {
		return rawURL, s.signErr
	}
	if s.signed != "" {
		return s.signed, nil
	}
	return rawURL, nil
}

func (s *stubBilibiliSession) InvalidateWBI() {}

type stubGrantView struct {
	capabilities map[string]bool
	httpHosts    []string
}

func (s *stubGrantView) CapabilityGranted(_ context.Context, _ string, capability string) bool {
	return s.capabilities[capability]
}

func (s *stubGrantView) StorageRootGranted(context.Context, string, string) bool {
	return false
}

func (s *stubGrantView) GrantedHTTPHosts(context.Context, string) []string {
	return append([]string(nil), s.httpHosts...)
}

func (s *stubGrantView) GrantedWebhookScope(context.Context, string, string) (plugins.WebhookScope, bool) {
	return plugins.WebhookScope{}, false
}

func (s *stubGrantView) ListPluginSnapshots() []plugins.Snapshot {
	return nil
}
