package actions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

func TestThirdPartyAccountReadReturnsDeclaredPlatformAccounts(t *testing.T) {
	t.Parallel()

	checkedAt := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	result, err := executeThirdPartyAccountRead(context.Background(), Deps{
		Capabilities: stubThirdPartyCapabilityView{
			capabilities: map[string]bool{"thirdparty.account.read": true},
			platforms:    []string{thirdparty.PlatformBilibili},
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
		Action: pluginruntime.Action{
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
		Capabilities: stubThirdPartyCapabilityView{
			capabilities: map[string]bool{"thirdparty.account.read": true},
			platforms:    []string{thirdparty.PlatformWeibo},
		},
		ThirdParty: stubThirdPartyAccountReader{},
	}, ActionRequest{
		PluginID: "raylea.subscription-hub",
		Action: pluginruntime.Action{
			Kind:                      "thirdparty.account.read",
			ThirdPartyAccountPlatform: thirdparty.PlatformBilibili,
		},
	})

	var runtimeErr *pluginruntime.Error
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("expected runtime error, got %#v", err)
	}
	if runtimeErr.Code != "plugin.capability_violation" {
		t.Fatalf("unexpected runtime error: %#v", runtimeErr)
	}
}

func TestThirdPartyAccountValidateQueuesAuthoritativeCheck(t *testing.T) {
	t.Parallel()

	requester := &stubThirdPartyAccountValidationRequester{}
	result, err := executeThirdPartyAccountValidate(context.Background(), Deps{
		Capabilities: stubThirdPartyCapabilityView{
			capabilities: map[string]bool{"thirdparty.account.validate": true},
			platforms:    []string{thirdparty.PlatformWeibo},
		},
		AccountValidation: requester,
	}, ActionRequest{
		PluginID: "raylea.subscription-hub",
		Action: pluginruntime.Action{
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

func TestThirdPartyAccountValidateRequiresSeparateCapability(t *testing.T) {
	t.Parallel()

	_, err := executeThirdPartyAccountValidate(context.Background(), Deps{
		Capabilities: stubThirdPartyCapabilityView{
			capabilities: map[string]bool{"thirdparty.account.read": true},
			platforms:    []string{thirdparty.PlatformWeibo},
		},
		AccountValidation: &stubThirdPartyAccountValidationRequester{},
	}, ActionRequest{
		PluginID: "raylea.subscription-hub",
		Action: pluginruntime.Action{
			Kind:                         "thirdparty.account.validate",
			ThirdPartyAccountPlatform:    thirdparty.PlatformWeibo,
			ThirdPartyAccountID:          "primary",
			ThirdPartyAccountObservation: "auth_rejected",
			ThirdPartyAccountHTTPStatus:  401,
		},
	})
	var runtimeErr *pluginruntime.Error
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "plugin.capability_violation" {
		t.Fatalf("expected capability violation, got %#v", err)
	}
}

type stubThirdPartyCapabilityView struct {
	capabilities map[string]bool
	platforms    []string
}

func (s stubThirdPartyCapabilityView) CapabilityDeclared(_ context.Context, _ string, capability string) bool {
	return s.capabilities[capability]
}

func (s stubThirdPartyCapabilityView) StorageRootAllowed(context.Context, string, string) bool {
	return false
}

func (s stubThirdPartyCapabilityView) HTTPHosts(context.Context, string) []string {
	return nil
}

func (s stubThirdPartyCapabilityView) ThirdPartyAccountPlatforms(context.Context, string) []string {
	return append([]string(nil), s.platforms...)
}

func (s stubThirdPartyCapabilityView) WebhookParameters(context.Context, string, string) (plugins.WebhookScope, bool) {
	return plugins.WebhookScope{}, false
}

func (s stubThirdPartyCapabilityView) ListPluginSnapshots() []plugins.Snapshot {
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
