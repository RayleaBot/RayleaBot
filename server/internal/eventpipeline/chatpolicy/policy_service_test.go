package chatpolicy

import (
	"context"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
)

func policyConfigWithUserLimit(limit string) config.Config {
	cfg := config.Config{}
	cfg.User.CommandRateLimit = limit
	return cfg
}

type stubBlacklistRepo struct {
	blockedType string
}

func (r *stubBlacklistRepo) IsBlacklisted(_ context.Context, entryType, _ string) (bool, error) {
	return entryType == r.blockedType, nil
}

func (r *stubBlacklistRepo) Get(context.Context, string, string) (permission.BlacklistEntry, error) {
	return permission.BlacklistEntry{}, permission.ErrGovernanceEntryNotFound
}

func (r *stubBlacklistRepo) Add(context.Context, string, string, string) error { return nil }

func (r *stubBlacklistRepo) Remove(context.Context, string, string) error { return nil }

func (r *stubBlacklistRepo) List(context.Context, string) ([]permission.BlacklistEntry, error) {
	return nil, nil
}

// Regression: the group-vs-user blacklist summary used to be derived by
// string-matching the user-facing Reason text instead of structured fields.
func TestCommandPolicyReasonSummaryDistinguishesBlacklistScope(t *testing.T) {
	cases := []struct {
		name        string
		blockedType string
		wantScope   string
		wantSummary string
	}{
		{name: "group blacklist", blockedType: "group", wantScope: permission.ScopeGroup, wantSummary: "群在黑名单中"},
		{name: "user blacklist", blockedType: "user", wantScope: permission.ScopeUser, wantSummary: "用户在黑名单中"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service := New(Deps{
				CurrentConfig: func() config.Config { return config.Config{} },
				BlacklistRepo: &stubBlacklistRepo{blockedType: testCase.blockedType},
			})
			verdict := service.PermissionChecker().Check(context.Background(), "1001", "member", "20001", nil)
			if verdict.Allowed {
				t.Fatal("expected blacklist rejection")
			}
			if verdict.Scope != testCase.wantScope {
				t.Fatalf("verdict scope = %q, want %q", verdict.Scope, testCase.wantScope)
			}
			if got := commandPolicyReasonSummary(verdict); got != testCase.wantSummary {
				t.Fatalf("reason summary = %q, want %q", got, testCase.wantSummary)
			}
		})
	}
}

// Regression: any config PUT used to rebuild the cooldown tracker and reset
// every in-flight rate-limit window.
func TestUpdateConfigPreservesCooldownWhenRateLimitsUnchanged(t *testing.T) {
	cfg := policyConfigWithUserLimit("1/60s")
	service := New(Deps{CurrentConfig: func() config.Config { return cfg }})

	cmd := &permission.CommandInfo{}
	if verdict := service.PermissionChecker().Check(context.Background(), "1001", "member", "", cmd); !verdict.Allowed {
		t.Fatalf("first command should be allowed, got %+v", verdict)
	}
	if verdict := service.PermissionChecker().Check(context.Background(), "1001", "member", "", cmd); verdict.Allowed {
		t.Fatal("second command should hit the rate limit")
	}

	next := policyConfigWithUserLimit("1/60s")
	next.Admin.SuperAdmins = []string{"9999"}
	service.UpdateConfig(next)

	if verdict := service.PermissionChecker().Check(context.Background(), "1001", "member", "", cmd); verdict.Allowed {
		t.Fatal("cooldown window should survive a config update that keeps rate limits unchanged")
	}
}

func TestUpdateConfigResetsCooldownWhenRateLimitsChange(t *testing.T) {
	cfg := policyConfigWithUserLimit("1/60s")
	service := New(Deps{CurrentConfig: func() config.Config { return cfg }})

	cmd := &permission.CommandInfo{}
	service.PermissionChecker().Check(context.Background(), "1001", "member", "", cmd)
	if verdict := service.PermissionChecker().Check(context.Background(), "1001", "member", "", cmd); verdict.Allowed {
		t.Fatal("second command should hit the rate limit before the update")
	}

	service.UpdateConfig(policyConfigWithUserLimit("5/60s"))

	if verdict := service.PermissionChecker().Check(context.Background(), "1001", "member", "", cmd); !verdict.Allowed {
		t.Fatalf("changed rate limit should rebuild the tracker, got %+v", verdict)
	}
}
