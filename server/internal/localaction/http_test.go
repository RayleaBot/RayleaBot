package localaction

import (
	"context"
	"errors"
	"testing"

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
	service := &Service{thirdParty: accounts}
	headers := map[string]string{"Accept": "application/json"}

	usedAccount, applied := service.applyBilibiliCookie(context.Background(), subscriptionHubPluginID, "https://api.bilibili.com/x/web-interface/nav", []string{"api.bilibili.com"}, headers)

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

func TestApplyBilibiliCookieSkipsNonBilibiliHosts(t *testing.T) {
	t.Parallel()

	service := &Service{thirdParty: &stubThirdPartyAccounts{
		accounts: []thirdparty.Account{{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}},
		cookies:  map[string]string{"primary": "SESSDATA=fixture;"},
	}}
	headers := map[string]string{}

	_, applied := service.applyBilibiliCookie(context.Background(), subscriptionHubPluginID, "https://example.com/api", []string{"example.com"}, headers)

	if applied {
		t.Fatalf("unexpected cookie application for non-Bilibili host")
	}
	if _, ok := headers["Cookie"]; ok {
		t.Fatalf("unexpected Cookie header: %#v", headers)
	}
}

func TestApplyBilibiliCookieKeepsPluginCookie(t *testing.T) {
	t.Parallel()

	service := &Service{thirdParty: &stubThirdPartyAccounts{
		accounts: []thirdparty.Account{{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}},
		cookies:  map[string]string{"primary": "SESSDATA=fixture;"},
	}}
	headers := map[string]string{"cookie": "SESSDATA=plugin;"}

	_, applied := service.applyBilibiliCookie(context.Background(), subscriptionHubPluginID, "https://api.live.bilibili.com/room/v1/Room/get_info", []string{"api.live.bilibili.com"}, headers)

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
	service := &Service{thirdParty: accounts}
	headers := map[string]string{}

	_, applied := service.applyBilibiliCookie(context.Background(), subscriptionHubPluginID, "https://api.bilibili.com/x/web-interface/nav", []string{"example.com"}, headers)

	if applied {
		t.Fatalf("unexpected cookie application without granted host")
	}
	if _, ok := headers["Cookie"]; ok {
		t.Fatalf("unexpected Cookie header: %#v", headers)
	}
}

func TestApplyBilibiliCookieRequiresSubscriptionHubPlugin(t *testing.T) {
	t.Parallel()

	service := &Service{thirdParty: &stubThirdPartyAccounts{
		accounts: []thirdparty.Account{{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}},
		cookies:  map[string]string{"primary": "SESSDATA=fixture;"},
	}}
	headers := map[string]string{}

	_, applied := service.applyBilibiliCookie(context.Background(), "raylea.other-plugin", "https://api.bilibili.com/x/web-interface/nav", []string{"api.bilibili.com"}, headers)

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
			if got := isBilibiliURL(tt.rawURL); got != tt.want {
				t.Fatalf("isBilibiliURL(%q) = %v, want %v", tt.rawURL, got, tt.want)
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
