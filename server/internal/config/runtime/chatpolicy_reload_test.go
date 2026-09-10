package runtime_test

import (
	"context"
	"testing"

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
			KVValueMaxBytes:          1024,
			KVTotalLimitMB:           8,
			FileMaxBytes:             2048,
			PluginWorkDirSoftLimitMB: 32,
		},
		HTTP: config.HTTPConfig{
			TimeoutSeconds:    10,
			MaxRetries:        0,
			AllowPrivateHosts: []string{},
		},
		Log: config.LogConfig{Level: "info"},
		Message: config.MessageConfig{
			RateLimitPerPlugin:    "1/1h",
			RateLimitPerTarget:    "100/1s",
			CircuitBreakerSeconds: 1,
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
			KVValueMaxBytes:          4096,
			KVTotalLimitMB:           16,
			FileMaxBytes:             8192,
			PluginWorkDirSoftLimitMB: 64,
		},
		HTTP: config.HTTPConfig{
			TimeoutSeconds:    15,
			MaxRetries:        2,
			AllowPrivateHosts: []string{"127.0.0.1"},
		},
		Log: config.LogConfig{Level: "info"},
		Message: config.MessageConfig{
			RateLimitPerPlugin:    "2/1h",
			RateLimitPerTarget:    "100/1s",
			CircuitBreakerSeconds: 1,
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
	if cfg.Storage.FileMaxBytes != 8192 || cfg.Storage.PluginWorkDirSoftLimitMB != 64 {
		t.Fatalf("storage config was not hot reloaded: %+v", cfg.Storage)
	}
	if cfg.HTTP.TimeoutSeconds != 15 || cfg.HTTP.MaxRetries != 2 {
		t.Fatalf("http config was not hot reloaded: %+v", cfg.HTTP)
	}
	if len(cfg.HTTP.AllowPrivateHosts) != 1 || cfg.HTTP.AllowPrivateHosts[0] != "127.0.0.1" {
		t.Fatalf("http allow_private_hosts was not hot reloaded: %+v", cfg.HTTP.AllowPrivateHosts)
	}
	if err := limiter.Wait(context.Background(), outbound.MessageLimitRequest{
		PluginID:   "weather",
		TargetType: "group",
		TargetID:   "20002",
	}); err != nil {
		t.Fatalf("new outbound message limit was not applied: %v", err)
	}
}
