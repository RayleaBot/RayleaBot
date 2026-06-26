package defaultmodules

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

func TestThirdPartyAccountReadReturnsDeclaredPlatformAccounts(t *testing.T) {
	t.Parallel()

	checkedAt := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	result, err := executeThirdPartyAccountRead(context.Background(), actions.Deps{
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
	}, actions.ActionRequest{
		PluginID: "raylea.subscription-hub",
		Action: runtimeaction.Action{
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

	_, err := executeThirdPartyAccountRead(context.Background(), actions.Deps{
		Capabilities: stubThirdPartyCapabilityView{
			capabilities: map[string]bool{"thirdparty.account.read": true},
			platforms:    []string{thirdparty.PlatformWeibo},
		},
		ThirdParty: stubThirdPartyAccountReader{},
	}, actions.ActionRequest{
		PluginID: "raylea.subscription-hub",
		Action: runtimeaction.Action{
			Kind:                      "thirdparty.account.read",
			ThirdPartyAccountPlatform: thirdparty.PlatformBilibili,
		},
	})

	var runtimeErr *runtimemanager.Error
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("expected runtime error, got %#v", err)
	}
	if runtimeErr.Code != "plugin.capability_violation" {
		t.Fatalf("unexpected runtime error: %#v", runtimeErr)
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

func (s stubThirdPartyAccountReader) ListEnabled(context.Context, string) ([]thirdparty.Account, error) {
	return append([]thirdparty.Account(nil), s.accounts...), nil
}

func (s stubThirdPartyAccountReader) ReadCookie(_ context.Context, account thirdparty.Account) (string, error) {
	return s.cookies[account.Platform+"/"+account.AccountID], nil
}
