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
