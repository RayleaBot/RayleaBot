package diagnostics

import (
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

func TestForStatusPrioritizesInvalidCredential(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 8, 8, 10, 0, 0, time.UTC)
	diagnosis := ForStatus(Status{
		Status: StateConnected,
		Live: LiveStatus{
			WatchedRooms: 1,
		},
		Dynamic: DynamicStatus{
			Enabled:     true,
			WatchedUIDs: 1,
		},
		Accounts: []thirdparty.Account{{
			AccountID: "10001",
			Label:     "主账号",
			Credential: thirdparty.CredentialStatus{
				State:     thirdparty.CredentialInvalid,
				LastError: "账号未登录",
			},
		}},
	}, []Cooldown{{
		Scope:     CooldownScopeLive,
		Code:      "platform_risk_control",
		Until:     now.Add(time.Minute),
		LastError: "risk",
	}}, now)

	if diagnosis.Level != "action_required" || diagnosis.Headline != "CK 需要重新登录" {
		t.Fatalf("unexpected diagnosis summary: %#v", diagnosis)
	}
	if len(diagnosis.Causes) != 1 || diagnosis.Causes[0].Scope != "account" || diagnosis.Causes[0].Code != "credential_invalid" {
		t.Fatalf("unexpected causes: %#v", diagnosis.Causes)
	}
	if len(diagnosis.Actions) == 0 || diagnosis.Actions[0].Kind != "open_accounts" || diagnosis.Actions[0].Target == nil || *diagnosis.Actions[0].Target != "/third-party-accounts" {
		t.Fatalf("unexpected actions: %#v", diagnosis.Actions)
	}
	if !diagnosis.UpdatedAt.Equal(now) {
		t.Fatalf("updated_at = %s, want %s", diagnosis.UpdatedAt, now)
	}
}
