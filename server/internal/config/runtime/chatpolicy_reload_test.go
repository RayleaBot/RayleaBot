package runtime_test

import (
	"context"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/chatpolicy"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	configruntime "github.com/RayleaBot/RayleaBot/server/internal/config/runtime"
)

func TestApplyHotReloadableFieldsReloadsCommandPolicy(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Admin: config.AdminConfig{
			SuperAdmins: []string{"1"},
		},
		Permission: config.PermissionConfig{
			DefaultLevel: "everyone",
		},
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
		User: config.UserConfig{
			CommandRateLimit: "5/1h",
			CooldownReply:    false,
		},
		Group: config.GroupConfig{
			CommandRateLimit: "5/1h",
		},
		Storage: config.StorageConfig{
			KVValueMaxBytes: 1024,
			KVTotalLimitMB:  8,
		},
		Log: config.LogConfig{Level: "info"},
		Message: config.MessageConfig{
			RateLimitPerTarget: "1/1h",
		},
	}
	ingress := chatpolicy.NewIngress(chatpolicy.IngressDeps{CurrentConfig: func() config.Config { return cfg }})
	limiter := outbound.NewMessageRateLimiter(cfg)
	service := configruntime.NewService(configruntime.Deps{CurrentConfig: func() config.Config { return cfg }, SetConfig: func(next config.Config) { cfg = next }, EventIngress: ingress, OutboundLimiter: limiter})
	if err := limiter.Wait(context.Background(), outbound.MessageLimitRequest{
		PluginID:   "weather",
		TargetType: "group",
		TargetID:   "20001",
	}); err != nil {
		t.Fatalf("prime outbound limiter: %v", err)
	}

	restartRequired := service.ApplyHotReloadableFields(config.Config{
		Admin: config.AdminConfig{
			SuperAdmins: []string{"42"},
		},
		Permission: config.PermissionConfig{
			DefaultLevel: "group_admin",
		},
		Command: &config.CommandConfig{
			Prefixes: []string{"!"},
		},
		User: config.UserConfig{
			CommandRateLimit: "1/1h",
			CooldownReply:    true,
		},
		Group: config.GroupConfig{
			CommandRateLimit: "2/1h",
		},
		Storage: config.StorageConfig{
			KVValueMaxBytes: 4096,
			KVTotalLimitMB:  16,
		},
		Log: config.LogConfig{Level: "info"},
		Message: config.MessageConfig{
			RateLimitPerTarget: "2/1h",
		},
	}).RestartRequired()
	if restartRequired {
		t.Fatal("restartRequired = true, want false for hot-reloadable fields")
	}
	if !ingress.Policy().CommandParser().Parse("!ping").IsCommand {
		t.Fatal("new command prefix was not applied")
	}
	if ingress.Policy().CommandParser().Parse("/ping").IsCommand {
		t.Fatal("old command prefix should no longer be active")
	}
	if verdict := ingress.Policy().PermissionChecker().Check(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "42", "member", "", &permission.CommandInfo{Permission: "super_admin"}); !verdict.Allowed {
		t.Fatalf("new super admin should bypass command checks: %#v", verdict)
	}
	if verdict := ingress.Policy().PermissionChecker().Check(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "1", "member", "", &permission.CommandInfo{Permission: "super_admin"}); verdict.Allowed {
		t.Fatalf("old super admin should no longer bypass command checks: %#v", verdict)
	}
	if cfg.Storage.KVValueMaxBytes != 4096 || cfg.Storage.KVTotalLimitMB != 16 {
		t.Fatalf("storage config was not hot reloaded: %+v", cfg.Storage)
	}
	waitCtx, cancelWait := context.WithTimeout(context.Background(), time.Second)
	defer cancelWait()
	if err := limiter.Wait(waitCtx, outbound.MessageLimitRequest{
		PluginID:   "weather",
		TargetType: "group",
		TargetID:   "20001",
	}); err != nil {
		t.Fatalf("new outbound message limit was not applied: %v", err)
	}
}
