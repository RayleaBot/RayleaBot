package actions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestThirdPartyAccountReadReturnsDeclaredPlatformAccounts(t *testing.T) {
	t.Parallel()

	checkedAt := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	result, err := executeThirdPartyAccountRead(context.Background(), Deps{
		Permissions: stubThirdPartyPermissionView{
			permissions: map[string]bool{"thirdparty.account.read": true},
			platforms:   []string{thirdparty.PlatformBilibili},
		},
		ThirdParty: stubThirdPartyAccountReader{
			accounts: []thirdparty.Account{{
				Platform:   thirdparty.PlatformBilibili,
				AccountID:  "primary",
				Label:      "主账号",
				Enabled:    true,
				Configured: true,
				Profile: thirdparty.AccountProfile{
					UID:       "123456",
					Nickname:  "测试账号",
					AvatarURL: "https://example.test/avatar.jpg",
				},
				Credential: thirdparty.CredentialStatus{State: thirdparty.CredentialValid, CheckedAt: &checkedAt},
			}},
			cookies: map[string]string{"bilibili/primary": "SESSDATA=fixture;"},
		},
	}, ActionRequest{
		PluginID: "raylea.subscription-hub",
		Action: plugins.Action{
			Kind:                      "thirdparty.account.read",
			ThirdPartyAccountPlatform: thirdparty.PlatformBilibili,
		},
	})
	if err != nil {
		t.Fatalf("thirdparty.account.read failed: %v", err)
	}
	if result["platform"] != thirdparty.PlatformBilibili {
		t.Fatalf("unexpected platform: %#v", result)
	}
	accounts, ok := result["accounts"].([]map[string]any)
	if !ok || len(accounts) != 1 {
		t.Fatalf("unexpected accounts result: %#v", result["accounts"])
	}
	cookie, ok := accounts[0]["cookie"].(map[string]any)
	if !ok || cookie["secret"] != true || cookie["value"] != "SESSDATA=fixture;" {
		t.Fatalf("unexpected secret cookie payload: %#v", accounts[0]["cookie"])
	}
}

func TestThirdPartyAccountReadRejectsUndeclaredPlatform(t *testing.T) {
	t.Parallel()

	_, err := executeThirdPartyAccountRead(context.Background(), Deps{
		Permissions: stubThirdPartyPermissionView{
			permissions: map[string]bool{"thirdparty.account.read": true},
			platforms:   []string{thirdparty.PlatformWeibo},
		},
		ThirdParty: stubThirdPartyAccountReader{},
	}, ActionRequest{
		PluginID: "raylea.subscription-hub",
		Action: plugins.Action{
			Kind:                      "thirdparty.account.read",
			ThirdPartyAccountPlatform: thirdparty.PlatformBilibili,
		},
	})

	var runtimeErr *plugins.Error
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("expected runtime error, got %#v", err)
	}
	if runtimeErr.Code != "plugin.permission_denied" {
		t.Fatalf("unexpected runtime error: %#v", runtimeErr)
	}
}

func TestThirdPartyAccountValidateQueuesAuthoritativeCheck(t *testing.T) {
	t.Parallel()

	requester := &stubThirdPartyAccountValidationRequester{}
	result, err := executeThirdPartyAccountValidate(context.Background(), Deps{
		Permissions: stubThirdPartyPermissionView{
			permissions: map[string]bool{"thirdparty.account.validate": true},
			platforms:   []string{thirdparty.PlatformWeibo},
		},
		AccountValidation: requester,
	}, ActionRequest{
		PluginID: "raylea.subscription-hub",
		Action: plugins.Action{
			Kind:                         "thirdparty.account.validate",
			ThirdPartyAccountPlatform:    thirdparty.PlatformWeibo,
			ThirdPartyAccountID:          "primary",
			ThirdPartyAccountObservation: "session_blocked",
			ThirdPartyAccountHTTPStatus:  432,
		},
	})
	if err != nil {
		t.Fatalf("thirdparty.account.validate failed: %v", err)
	}
	if result["accepted"] != true || result["reason"] != "queued" {
		t.Fatalf("unexpected validation result: %#v", result)
	}
	if requester.pluginID != "raylea.subscription-hub" || requester.platform != thirdparty.PlatformWeibo || requester.accountID != "primary" || requester.observation != "session_blocked" || requester.httpStatus != 432 {
		t.Fatalf("unexpected validation request: %#v", requester)
	}
}

func TestThirdPartyAccountValidateRequiresSeparatePermission(t *testing.T) {
	t.Parallel()

	_, err := executeThirdPartyAccountValidate(context.Background(), Deps{
		Permissions: stubThirdPartyPermissionView{
			permissions: map[string]bool{"thirdparty.account.read": true},
			platforms:   []string{thirdparty.PlatformWeibo},
		},
		AccountValidation: &stubThirdPartyAccountValidationRequester{},
	}, ActionRequest{
		PluginID: "raylea.subscription-hub",
		Action: plugins.Action{
			Kind:                         "thirdparty.account.validate",
			ThirdPartyAccountPlatform:    thirdparty.PlatformWeibo,
			ThirdPartyAccountID:          "primary",
			ThirdPartyAccountObservation: "auth_rejected",
			ThirdPartyAccountHTTPStatus:  401,
		},
	})
	var runtimeErr *plugins.Error
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "plugin.permission_denied" {
		t.Fatalf("expected permission violation, got %#v", err)
	}
}

func TestThirdPartyResolvePassesAccountCookiesToBrowserResolver(t *testing.T) {
	t.Parallel()

	resolver := &stubThirdPartyResolver{
		profiles: []thirdparty.AccountProfile{{
			UID:       "MS4wLjABAAAAhost",
			Nickname:  "洛天依",
			AvatarURL: "https://p3-pc.douyinpic.com/host.jpeg",
		}},
		exact: true,
	}
	result, err := executeThirdPartyResolve(context.Background(), Deps{
		Permissions: stubThirdPartyPermissionView{
			permissions: map[string]bool{"thirdparty.resolve": true},
			platforms:   []string{thirdparty.PlatformDouyin},
		},
		ThirdParty: stubThirdPartyAccountReader{
			accounts: []thirdparty.Account{{
				Platform:   thirdparty.PlatformDouyin,
				AccountID:  "primary",
				Enabled:    true,
				Configured: true,
			}},
			cookies: map[string]string{"douyin/primary": "sessionid=fixture; ttwid=fixture;"},
		},
		ThirdPartyResolve: resolver,
	}, ActionRequest{
		PluginID: "raylea.subscription-hub",
		Action: plugins.Action{
			Kind:                      "thirdparty.resolve",
			ThirdPartyAccountPlatform: thirdparty.PlatformDouyin,
			ThirdPartyResolveQuery:    "  洛天依  ",
		},
	})
	if err != nil {
		t.Fatalf("thirdparty.resolve failed: %v", err)
	}
	if resolver.query != "洛天依" {
		t.Fatalf("resolver query = %q, want trimmed keyword", resolver.query)
	}
	if len(resolver.cookieSets) != 1 || resolver.cookieSets[0]["sessionid"] != "fixture" {
		t.Fatalf("resolver cookie sets = %#v", resolver.cookieSets)
	}
	if result["platform"] != thirdparty.PlatformDouyin || result["exact"] != true {
		t.Fatalf("unexpected resolve result: %#v", result)
	}
	profiles, ok := result["profiles"].([]map[string]any)
	if !ok || len(profiles) != 1 {
		t.Fatalf("unexpected profiles result: %#v", result["profiles"])
	}
	if profiles[0]["uid"] != "MS4wLjABAAAAhost" || profiles[0]["nickname"] != "洛天依" || profiles[0]["avatar_url"] != "https://p3-pc.douyinpic.com/host.jpeg" {
		t.Fatalf("unexpected profile payload: %#v", profiles[0])
	}
}

func TestThirdPartyResolveMergesRequestCookie(t *testing.T) {
	t.Parallel()

	resolver := &stubThirdPartyResolver{
		profiles: []thirdparty.AccountProfile{{UID: "MS4wLjABAAAAhost", Nickname: "洛天依"}},
		exact:    true,
	}
	result, err := executeThirdPartyResolve(context.Background(), Deps{
		Permissions: stubThirdPartyPermissionView{
			permissions: map[string]bool{"thirdparty.resolve": true},
			platforms:   []string{thirdparty.PlatformDouyin},
		},
		ThirdParty: stubThirdPartyAccountReader{
			accounts: []thirdparty.Account{{
				Platform:   thirdparty.PlatformDouyin,
				AccountID:  "primary",
				Enabled:    true,
				Configured: true,
			}},
			cookies: map[string]string{"douyin/primary": "sessionid=store; ttwid=store;"},
		},
		ThirdPartyResolve: resolver,
	}, ActionRequest{
		PluginID: "raylea.subscription-hub",
		Action: plugins.Action{
			Kind:                      "thirdparty.resolve",
			ThirdPartyAccountPlatform: thirdparty.PlatformDouyin,
			ThirdPartyResolveQuery:    "洛天依",
			ThirdPartyResolveCookie:   "sessionid=req; odin_tt=req;",
		},
	})
	if err != nil {
		t.Fatalf("thirdparty.resolve failed: %v", err)
	}
	if len(resolver.cookieSets) != 2 {
		t.Fatalf("resolver cookie sets = %d, want store account plus request cookie", len(resolver.cookieSets))
	}
	// store CK 先注入作为基线，插件显式 CK 后注入并覆盖同名字段。
	if resolver.cookieSets[0]["sessionid"] != "store" || resolver.cookieSets[0]["ttwid"] != "store" {
		t.Fatalf("store account cookie set = %#v", resolver.cookieSets[0])
	}
	if resolver.cookieSets[1]["sessionid"] != "req" || resolver.cookieSets[1]["odin_tt"] != "req" {
		t.Fatalf("request cookie set = %#v", resolver.cookieSets[1])
	}
	if result["exact"] != true {
		t.Fatalf("unexpected resolve result: %#v", result)
	}
}

func TestThirdPartyResolveRejectsUnsupportedPlatform(t *testing.T) {
	t.Parallel()

	_, err := executeThirdPartyResolve(context.Background(), Deps{
		Permissions: stubThirdPartyPermissionView{
			permissions: map[string]bool{"thirdparty.resolve": true},
			platforms:   []string{thirdparty.PlatformBilibili},
		},
		ThirdPartyResolve: &stubThirdPartyResolver{},
	}, ActionRequest{
		PluginID: "raylea.subscription-hub",
		Action: plugins.Action{
			Kind:                      "thirdparty.resolve",
			ThirdPartyAccountPlatform: thirdparty.PlatformBilibili,
			ThirdPartyResolveQuery:    "测试用户",
		},
	})
	var runtimeErr *plugins.Error
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "platform.invalid_request" {
		t.Fatalf("expected platform.invalid_request, got %#v", err)
	}
}

func TestThirdPartyResolveRequiresPermission(t *testing.T) {
	t.Parallel()

	_, err := executeThirdPartyResolve(context.Background(), Deps{
		Permissions: stubThirdPartyPermissionView{
			permissions: map[string]bool{"thirdparty.account.read": true},
			platforms:   []string{thirdparty.PlatformDouyin},
		},
		ThirdPartyResolve: &stubThirdPartyResolver{},
	}, ActionRequest{
		PluginID: "raylea.subscription-hub",
		Action: plugins.Action{
			Kind:                      "thirdparty.resolve",
			ThirdPartyAccountPlatform: thirdparty.PlatformDouyin,
			ThirdPartyResolveQuery:    "测试用户",
		},
	})
	var runtimeErr *plugins.Error
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "plugin.permission_denied" {
		t.Fatalf("expected permission violation, got %#v", err)
	}
}

func TestThirdPartyResolveRejectsEmptyQuery(t *testing.T) {
	t.Parallel()

	_, err := executeThirdPartyResolve(context.Background(), Deps{
		Permissions: stubThirdPartyPermissionView{
			permissions: map[string]bool{"thirdparty.resolve": true},
			platforms:   []string{thirdparty.PlatformDouyin},
		},
		ThirdPartyResolve: &stubThirdPartyResolver{},
	}, ActionRequest{
		PluginID: "raylea.subscription-hub",
		Action: plugins.Action{
			Kind:                      "thirdparty.resolve",
			ThirdPartyAccountPlatform: thirdparty.PlatformDouyin,
			ThirdPartyResolveQuery:    "   ",
		},
	})
	var runtimeErr *plugins.Error
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "platform.invalid_request" {
		t.Fatalf("expected platform.invalid_request, got %#v", err)
	}
}

type stubThirdPartyPermissionView struct {
	permissions map[string]bool
	platforms   []string
}

func (s stubThirdPartyPermissionView) PermissionDeclared(_ context.Context, _ string, permission string) bool {
	return s.permissions[permission]
}

func (s stubThirdPartyPermissionView) PermissionPlatforms(context.Context, string, string) []string {
	return append([]string(nil), s.platforms...)
}

func (s stubThirdPartyPermissionView) ListPluginSnapshots() []plugins.Snapshot {
	return nil
}

type stubThirdPartyAccountReader struct {
	accounts []thirdparty.Account
	cookies  map[string]string
}

type stubThirdPartyAccountValidationRequester struct {
	pluginID    string
	platform    string
	accountID   string
	observation string
	httpStatus  int
}

func (s *stubThirdPartyAccountValidationRequester) RequestPluginValidation(_ context.Context, pluginID, platform, accountID, observation string, httpStatus int) (bool, string, error) {
	s.pluginID = pluginID
	s.platform = platform
	s.accountID = accountID
	s.observation = observation
	s.httpStatus = httpStatus
	return true, "queued", nil
}

func (s stubThirdPartyAccountReader) ListEnabled(context.Context, string) ([]thirdparty.Account, error) {
	return append([]thirdparty.Account(nil), s.accounts...), nil
}

func (s stubThirdPartyAccountReader) ReadCookie(_ context.Context, account thirdparty.Account) (string, error) {
	return s.cookies[account.Platform+"/"+account.AccountID], nil
}

type stubThirdPartyResolver struct {
	query      string
	cookieSets []map[string]string
	profiles   []thirdparty.AccountProfile
	exact      bool
	err        error
}

func (s *stubThirdPartyResolver) ResolveUser(_ context.Context, query string, cookieSets []map[string]string) ([]thirdparty.AccountProfile, bool, error) {
	s.query = query
	s.cookieSets = cookieSets
	return s.profiles, s.exact, s.err
}
