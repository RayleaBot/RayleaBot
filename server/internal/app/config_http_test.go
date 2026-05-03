package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
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

func TestHandleConfigPutHotReloadsOutboundLimiterMessageFields(t *testing.T) {
	tests := []struct {
		name        string
		baseMessage config.MessageConfig
		mutate      func(*testing.T, map[string]any)
		prime       outbound.MessageLimitRequest
		verify      func(*testing.T, *recordingConfigOutboundLimiter)
		wantPath    string
		wantConfig  func(config.Config) bool
	}{
		{
			name: "rate_limit_per_plugin",
			baseMessage: config.MessageConfig{
				RateLimitPerPlugin:    "1/1h",
				RateLimitPerTarget:    "100/1s",
				CircuitBreakerSeconds: 1,
			},
			mutate: func(t *testing.T, document map[string]any) {
				messageSection(t, document)["rate_limit_per_plugin"] = "2/1h"
			},
			prime: outbound.MessageLimitRequest{PluginID: "weather", TargetType: "group", TargetID: "100"},
			verify: func(t *testing.T, limiter *recordingConfigOutboundLimiter) {
				t.Helper()
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()
				if err := limiter.Wait(ctx, outbound.MessageLimitRequest{PluginID: "weather", TargetType: "group", TargetID: "101"}); err != nil {
					t.Fatalf("updated plugin rate limit was not applied to outbound limiter: %v", err)
				}
			},
			wantPath: "message.rate_limit_per_plugin",
			wantConfig: func(cfg config.Config) bool {
				return cfg.Message.RateLimitPerPlugin == "2/1h"
			},
		},
		{
			name: "rate_limit_per_target",
			baseMessage: config.MessageConfig{
				RateLimitPerPlugin:    "100/1s",
				RateLimitPerTarget:    "1/1h",
				CircuitBreakerSeconds: 1,
			},
			mutate: func(t *testing.T, document map[string]any) {
				messageSection(t, document)["rate_limit_per_target"] = "2/1h"
			},
			prime: outbound.MessageLimitRequest{PluginID: "weather", TargetType: "group", TargetID: "100"},
			verify: func(t *testing.T, limiter *recordingConfigOutboundLimiter) {
				t.Helper()
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()
				if err := limiter.Wait(ctx, outbound.MessageLimitRequest{PluginID: "news", TargetType: "group", TargetID: "100"}); err != nil {
					t.Fatalf("updated target rate limit was not applied to outbound limiter: %v", err)
				}
			},
			wantPath: "message.rate_limit_per_target",
			wantConfig: func(cfg config.Config) bool {
				return cfg.Message.RateLimitPerTarget == "2/1h"
			},
		},
		{
			name: "circuit_breaker_seconds",
			baseMessage: config.MessageConfig{
				RateLimitPerPlugin:    "100/1s",
				RateLimitPerTarget:    "1/1h",
				CircuitBreakerSeconds: 1,
			},
			mutate: func(t *testing.T, document map[string]any) {
				messageSection(t, document)["circuit_breaker_seconds"] = 3
			},
			prime: outbound.MessageLimitRequest{PluginID: "weather", TargetType: "group", TargetID: "100"},
			verify: func(t *testing.T, limiter *recordingConfigOutboundLimiter) {
				t.Helper()
				ctx, cancel := context.WithCancel(context.Background())
				done := make(chan error, 1)
				go func() {
					done <- limiter.Wait(ctx, outbound.MessageLimitRequest{PluginID: "news", TargetType: "group", TargetID: "100"})
				}()

				select {
				case err := <-done:
					t.Fatalf("outbound wait ended before the updated circuit breaker window: %v", err)
				case <-time.After(1300 * time.Millisecond):
				}

				cancel()
				select {
				case <-done:
				case <-time.After(200 * time.Millisecond):
					t.Fatal("outbound wait did not stop after test context was cancelled")
				}
			},
			wantPath: "message.circuit_breaker_seconds",
			wantConfig: func(cfg config.Config) bool {
				return cfg.Message.CircuitBreakerSeconds == 3
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, limiter := newConfigHTTPOutboundLimiterFixture(t, tt.baseMessage)
			if err := limiter.Wait(context.Background(), tt.prime); err != nil {
				t.Fatalf("prime outbound limiter: %v", err)
			}

			document := configDocumentFromTyped(app.state.Config)
			tt.mutate(t, document)
			response := putConfigDocument(t, app, limiter, document)

			if response.RestartRequired {
				t.Fatalf("restart_required = true, want false")
			}
			if !reflect.DeepEqual(response.ApplyEffects.AppliedNow, []string{tt.wantPath}) {
				t.Fatalf("applied_now = %#v, want [%s]", response.ApplyEffects.AppliedNow, tt.wantPath)
			}
			if len(limiter.applied) != 1 {
				t.Fatalf("outbound limiter ApplyConfig calls = %d, want 1", len(limiter.applied))
			}
			if !tt.wantConfig(limiter.applied[0]) {
				t.Fatalf("outbound limiter received config: %+v", limiter.applied[0].Message)
			}
			if !tt.wantConfig(app.state.Config) {
				t.Fatalf("state config was not updated: %+v", app.state.Config.Message)
			}
			tt.verify(t, limiter)
		})
	}
}

type recordingConfigOutboundLimiter struct {
	inner   *outbound.MessageRateLimiter
	applied []config.Config
}

func newRecordingConfigOutboundLimiter(cfg config.Config) *recordingConfigOutboundLimiter {
	return &recordingConfigOutboundLimiter{inner: outbound.NewMessageRateLimiter(cfg)}
}

func (l *recordingConfigOutboundLimiter) ApplyConfig(cfg config.Config) {
	l.applied = append(l.applied, cfg)
	l.inner.ApplyConfig(cfg)
}

func (l *recordingConfigOutboundLimiter) Wait(ctx context.Context, request outbound.MessageLimitRequest) error {
	return l.inner.Wait(ctx, request)
}

func newConfigHTTPOutboundLimiterFixture(t *testing.T, message config.MessageConfig) (*App, *recordingConfigOutboundLimiter) {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "user.yaml")
	schemaPath := configHTTPTestSchemaPath(t)
	cfg, summary, err := config.Load(configPath, schemaPath)
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}

	document := configDocumentFromTyped(cfg)
	messageDoc := messageSection(t, document)
	messageDoc["rate_limit_per_plugin"] = message.RateLimitPerPlugin
	messageDoc["rate_limit_per_target"] = message.RateLimitPerTarget
	messageDoc["circuit_breaker_seconds"] = message.CircuitBreakerSeconds
	cfg, summary, err = config.SaveDocument(configPath, schemaPath, document)
	if err != nil {
		t.Fatalf("save base config: %v", err)
	}

	app := newTestAppState(cfg, nil)
	app.state.Summary = summary
	limiter := newRecordingConfigOutboundLimiter(cfg)
	return app, limiter
}

func putConfigDocument(t *testing.T, app *App, limiter *recordingConfigOutboundLimiter, document map[string]any) configUpdateResponse {
	t.Helper()

	body, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("marshal config request: %v", err)
	}

	handler := newConfigHTTPHandlers(configHTTPDeps{
		state:           app.state,
		outboundLimiter: limiter,
	})
	request := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	handler.handleConfigPut().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("PUT /api/config status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var response configUpdateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode config response: %v", err)
	}
	return response
}

func configHTTPTestSchemaPath(t *testing.T) string {
	t.Helper()

	path, err := filepath.Abs(filepath.Join("..", "..", "..", "contracts", "config.user.schema.json"))
	if err != nil {
		t.Fatalf("resolve config schema path: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat config schema %s: %v", path, err)
	}
	return path
}

func messageSection(t *testing.T, document map[string]any) map[string]any {
	t.Helper()

	message, ok := document["message"].(map[string]any)
	if !ok {
		t.Fatalf("document message section = %#v, want map[string]any", document["message"])
	}
	return message
}
