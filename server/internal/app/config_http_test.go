package app

import (
	"context"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestApplyHotReloadableFieldsClassifiesCanonicalPaths(t *testing.T) {
	t.Parallel()

	app := newTestAppState(config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		OneBot: config.OneBotConfig{
			Provider: "standard",
			ReverseWS: config.OneBotTransportConfig{
				Enabled: false,
				URL:     "",
			},
			ForwardWS: config.OneBotTransportConfig{
				Enabled: false,
				URL:     "",
			},
			HTTPAPI: config.OneBotTransportConfig{
				Enabled: false,
				URL:     "",
			},
			Webhook: config.OneBotTransportConfig{
				Enabled: false,
				URL:     "",
			},
		},
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
		Permission: config.PermissionConfig{
			DefaultLevel: "everyone",
		},
		Render: config.RenderConfig{
			WorkerCount:             1,
			BrowserArgs:             []string{"--disable-gpu"},
			BrowserPath:             "",
			TimeoutSeconds:          30,
			QueueWaitTimeoutSeconds: 15,
			QueueMaxLength:          32,
		},
		Log: config.LogConfig{
			Level:              "info",
			RetentionDays:      7,
			RateLimitPerPlugin: "200/10s",
		},
		User: config.UserConfig{
			CommandRateLimit: "10/60s",
			CooldownReply:    true,
		},
		Adapter: config.AdapterConfig{
			ConnectTimeoutSeconds:   15,
			ReconnectInitialSeconds: 2,
			ReconnectMultiplier:     2,
			ReconnectMaxSeconds:     120,
			ReconnectJitterRatio:    0.2,
		},
	}, nil)

	effects := applyConfigApplyEffects(app, config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 8081,
		},
		OneBot: config.OneBotConfig{
			Provider: "standard",
			ReverseWS: config.OneBotTransportConfig{
				Enabled: false,
				URL:     "",
			},
			ForwardWS: config.OneBotTransportConfig{
				Enabled: true,
				URL:     "ws://127.0.0.1:2658",
			},
			HTTPAPI: config.OneBotTransportConfig{
				Enabled: false,
				URL:     "",
			},
			Webhook: config.OneBotTransportConfig{
				Enabled: false,
				URL:     "",
			},
		},
		Command: &config.CommandConfig{
			Prefixes: []string{"!"},
		},
		Permission: config.PermissionConfig{
			DefaultLevel: "group_admin",
		},
		Render: config.RenderConfig{
			WorkerCount:             1,
			BrowserArgs:             []string{"--disable-gpu", "--headless=new"},
			BrowserPath:             "",
			TimeoutSeconds:          30,
			QueueWaitTimeoutSeconds: 15,
			QueueMaxLength:          32,
		},
		Log: config.LogConfig{
			Level:              "debug",
			RetentionDays:      7,
			RateLimitPerPlugin: "200/10s",
		},
		User: config.UserConfig{
			CommandRateLimit: "1/1h",
			CooldownReply:    true,
		},
		Adapter: config.AdapterConfig{
			ConnectTimeoutSeconds:   20,
			ReconnectInitialSeconds: 2,
			ReconnectMultiplier:     2,
			ReconnectMaxSeconds:     120,
			ReconnectJitterRatio:    0.2,
		},
	})

	if !reflect.DeepEqual(effects.AppliedNow, []string{
		"command.prefixes",
		"log.level",
		"permission.default_level",
		"user.command_rate_limit",
	}) {
		t.Fatalf("unexpected applied_now: %#v", effects.AppliedNow)
	}
	if !reflect.DeepEqual(effects.ReloadedNow, []string{
		"adapter.connect_timeout_seconds",
		"onebot.forward_ws.enabled",
		"onebot.forward_ws.url",
	}) {
		t.Fatalf("unexpected reloaded_now: %#v", effects.ReloadedNow)
	}
	if !reflect.DeepEqual(effects.RestartRequiredFields, []string{
		"render.browser_args",
		"server.port",
	}) {
		t.Fatalf("unexpected restart_required_fields: %#v", effects.RestartRequiredFields)
	}
	if !effects.restartRequired() {
		t.Fatal("restartRequired = false, want true")
	}
}

func TestApplyHotReloadableFieldsFallsBackToRestartRequiredWhenAdapterReloadFails(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	baseConfig := config.Config{
		OneBot: config.OneBotConfig{
			Provider: "standard",
			ReverseWS: config.OneBotTransportConfig{
				Enabled: false,
				URL:     "",
			},
			ForwardWS: config.OneBotTransportConfig{
				Enabled: false,
				URL:     "",
			},
			HTTPAPI: config.OneBotTransportConfig{
				Enabled: false,
				URL:     "",
			},
			Webhook: config.OneBotTransportConfig{
				Enabled: false,
				URL:     "",
			},
		},
		Adapter: config.AdapterConfig{
			ConnectTimeoutSeconds:   15,
			ReconnectInitialSeconds: 2,
			ReconnectMultiplier:     2,
			ReconnectMaxSeconds:     120,
			ReconnectJitterRatio:    0.2,
		},
	}
	app := newTestAppState(baseConfig, logger)

	adapterShell := adapter.New(baseConfig.OneBot, logger)
	startCtx, cancelStart := context.WithCancel(context.Background())
	adapterShell.Start(startCtx)
	cancelStart()
	app.protocol = newProtocolService(app.state, adapterShell)
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = adapterShell.Stop(stopCtx)
	})

	effects := applyConfigApplyEffects(app, config.Config{
		OneBot: config.OneBotConfig{
			Provider: "standard",
			ReverseWS: config.OneBotTransportConfig{
				Enabled: false,
				URL:     "",
			},
			ForwardWS: config.OneBotTransportConfig{
				Enabled: true,
				URL:     "ws://127.0.0.1:2658",
			},
			HTTPAPI: config.OneBotTransportConfig{
				Enabled: false,
				URL:     "",
			},
			Webhook: config.OneBotTransportConfig{
				Enabled: false,
				URL:     "",
			},
		},
		Adapter: baseConfig.Adapter,
	})

	if len(effects.ReloadedNow) != 0 {
		t.Fatalf("reloaded_now = %#v, want [] after reload failure", effects.ReloadedNow)
	}
	if !reflect.DeepEqual(effects.RestartRequiredFields, []string{
		"onebot.forward_ws.enabled",
		"onebot.forward_ws.url",
	}) {
		t.Fatalf("unexpected restart_required_fields after reload failure: %#v", effects.RestartRequiredFields)
	}
	if !effects.restartRequired() {
		t.Fatal("restartRequired = false, want true")
	}
}
