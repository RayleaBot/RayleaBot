package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"rayleabot/server/internal/config"
	"rayleabot/server/internal/dispatch"
	"rayleabot/server/internal/pluginconfig"
	"rayleabot/server/internal/pluginfile"
	"rayleabot/server/internal/pluginkv"
	"rayleabot/server/internal/plugins"
	"rayleabot/server/internal/runtime"
	"rayleabot/server/internal/scheduler"
	"rayleabot/server/internal/storage"
)

func TestExecuteLocalActionRejectsMissingCapability(t *testing.T) {
	t.Parallel()

	application := &App{
		appCore: appCore{
			Config: config.Config{},
			Logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		},
		appPlugins: appPlugins{
			grantRepository: &stubLifecycleGrantRepository{
				grants: map[string][]plugins.PluginGrant{},
			},
		},
	}
	application.pluginLifecycle = newPluginLifecycleController(application)

	_, err := application.executeLocalAction(context.Background(), "notice-logger", "req_local_1", runtime.Action{
		Kind:             "storage.kv",
		StorageOperation: "get",
		StorageKey:       "notice:last_join",
	})
	assertRuntimeErrorCode(t, err, "permission.scope_violation")
}

func TestExecuteLoggerWriteAppliesRateLimit(t *testing.T) {
	t.Parallel()

	buffer := &bytes.Buffer{}
	application := &App{
		appCore: appCore{
			Config: config.Config{
				Auth: config.AuthConfig{
					AutoGrantCapabilities: []string{"logger.write"},
				},
				Logging: config.LoggingConfig{
					RateLimitPerPlugin: "1/1h",
				},
			},
			Logger: slog.New(slog.NewJSONHandler(buffer, nil)),
			redactText: func(text string) string {
				return text
			},
		},
		appPlugins: appPlugins{
			pluginLogLimiter: newPluginLogLimiter(config.Config{Logging: config.LoggingConfig{RateLimitPerPlugin: "1/1h"}}),
		},
	}
	application.pluginLifecycle = newPluginLifecycleController(application)

	if _, err := application.executeLocalAction(context.Background(), "notice-logger", "req_local_2", runtime.Action{
		Kind:       "logger.write",
		LogLevel:   "info",
		LogMessage: "first log",
	}); err != nil {
		t.Fatalf("first logger.write failed: %v", err)
	}

	_, err := application.executeLocalAction(context.Background(), "notice-logger", "req_local_3", runtime.Action{
		Kind:       "logger.write",
		LogLevel:   "info",
		LogMessage: "second log",
	})
	assertRuntimeErrorCode(t, err, "platform.rate_limited")
}

func TestExecuteStorageKVRoundTrip(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer store.Close()

	repo, err := pluginkv.NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("NewSQLiteRepository: %v", err)
	}

	application := &App{
		appCore: appCore{
			Config: config.Config{
				Storage: config.StorageConfig{
					KVValueMaxBytes: 1024,
					KVTotalLimitMB:  1,
				},
			},
			Logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		},
		appPlugins: appPlugins{
			pluginKV: repo,
			grantRepository: &stubLifecycleGrantRepository{
				grants: map[string][]plugins.PluginGrant{
					"notice-logger": {{
						PluginID:   "notice-logger",
						Capability: "storage.kv",
					}},
				},
			},
		},
	}
	application.pluginLifecycle = newPluginLifecycleController(application)

	if _, err := application.executeLocalAction(context.Background(), "notice-logger", "req_local_4", runtime.Action{
		Kind:             "storage.kv",
		StorageOperation: "set",
		StorageKey:       "notice:last_join",
		StorageValue: map[string]any{
			"user_id": "3001",
			"count":   2,
		},
	}); err != nil {
		t.Fatalf("storage set failed: %v", err)
	}

	getResult, err := application.executeLocalAction(context.Background(), "notice-logger", "req_local_5", runtime.Action{
		Kind:             "storage.kv",
		StorageOperation: "get",
		StorageKey:       "notice:last_join",
	})
	if err != nil {
		t.Fatalf("storage get failed: %v", err)
	}
	if exists, _ := getResult["exists"].(bool); !exists {
		t.Fatalf("expected get exists=true, got %#v", getResult)
	}

	listResult, err := application.executeLocalAction(context.Background(), "notice-logger", "req_local_6", runtime.Action{
		Kind:             "storage.kv",
		StorageOperation: "list",
		StoragePrefix:    "notice:",
	})
	if err != nil {
		t.Fatalf("storage list failed: %v", err)
	}
	keys, _ := listResult["keys"].([]string)
	if len(keys) != 1 || keys[0] != "notice:last_join" {
		t.Fatalf("unexpected list keys: %#v", listResult["keys"])
	}

	deleteResult, err := application.executeLocalAction(context.Background(), "notice-logger", "req_local_7", runtime.Action{
		Kind:             "storage.kv",
		StorageOperation: "delete",
		StorageKey:       "notice:last_join",
	})
	if err != nil {
		t.Fatalf("storage delete failed: %v", err)
	}
	if deleted, _ := deleteResult["deleted"].(bool); !deleted {
		t.Fatalf("expected delete deleted=true, got %#v", deleteResult)
	}
}

func TestExecuteConfigReadWriteRoundTrip(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer store.Close()

	repo, err := pluginconfig.NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("NewSQLiteRepository: %v", err)
	}

	application := &App{
		appCore: appCore{
			Config: config.Config{
				Auth: config.AuthConfig{
					AutoGrantCapabilities: []string{"config.read", "config.write"},
				},
			},
			Logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		},
		appPlugins: appPlugins{
			pluginConfig:    repo,
			grantRepository: &stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
		},
	}
	application.pluginLifecycle = newPluginLifecycleController(application)

	if _, err := repo.SeedDefaults(context.Background(), "weather", map[string]any{
		"default_city": "北京",
		"unit":         "celsius",
	}); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}

	readResult, err := application.executeLocalAction(context.Background(), "weather", "req_config_1", runtime.Action{
		Kind:       "config.read",
		ConfigKeys: []string{"default_city", "unit", "missing"},
	})
	if err != nil {
		t.Fatalf("config.read failed: %v", err)
	}
	values, _ := readResult["values"].(map[string]any)
	if values["default_city"] != "北京" || values["unit"] != "celsius" {
		t.Fatalf("unexpected config.read values: %#v", values)
	}
	if _, ok := values["missing"]; ok {
		t.Fatalf("missing key should not be returned: %#v", values)
	}

	writeResult, err := application.executeLocalAction(context.Background(), "weather", "req_config_2", runtime.Action{
		Kind: "config.write",
		ConfigValues: map[string]any{
			"default_city": "上海",
			"unit":         "fahrenheit",
		},
	})
	if err != nil {
		t.Fatalf("config.write failed: %v", err)
	}
	changedKeys, _ := writeResult["changed_keys"].([]string)
	if len(changedKeys) != 2 || changedKeys[0] != "default_city" || changedKeys[1] != "unit" {
		t.Fatalf("unexpected changed_keys: %#v", writeResult["changed_keys"])
	}

	readResult, err = application.executeLocalAction(context.Background(), "weather", "req_config_3", runtime.Action{
		Kind:       "config.read",
		ConfigKeys: []string{"default_city", "unit"},
	})
	if err != nil {
		t.Fatalf("config.read second call failed: %v", err)
	}
	values, _ = readResult["values"].(map[string]any)
	if values["default_city"] != "上海" || values["unit"] != "fahrenheit" {
		t.Fatalf("unexpected updated config values: %#v", values)
	}
}

func TestExecuteConfigWriteDispatchesConfigChanged(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer store.Close()

	repo, err := pluginconfig.NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("NewSQLiteRepository: %v", err)
	}

	application := &App{
		appCore: appCore{
			Config: config.Config{
				Auth: config.AuthConfig{
					AutoGrantCapabilities: []string{"config.write"},
				},
			},
			Logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		},
		appPlugins: appPlugins{
			pluginConfig:    repo,
			Dispatcher:      dispatch.New(slog.Default(), nil, nil, 16),
			grantRepository: &stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
		},
	}
	application.pluginLifecycle = newPluginLifecycleController(application)
	fakeRuntime := &capturingRuntime{events: make(chan runtime.Event, 1)}
	application.Dispatcher.Register("weather", fakeRuntime, []string{"config.changed"}, nil)

	if _, err := application.executeLocalAction(context.Background(), "weather", "req_config_changed", runtime.Action{
		Kind: "config.write",
		ConfigValues: map[string]any{
			"default_city": "上海",
		},
	}); err != nil {
		t.Fatalf("config.write failed: %v", err)
	}

	select {
	case event := <-fakeRuntime.events:
		if event.EventType != "config.changed" {
			t.Fatalf("unexpected config.changed event: %#v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected config.changed event")
	}
}

func TestExecuteSchedulerCreateUpsert(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer store.Close()

	repo, err := scheduler.NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("NewSQLiteRepository: %v", err)
	}
	engine, err := scheduler.New(scheduler.Options{
		Repository: repo,
		Logger:     slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
	})
	if err != nil {
		t.Fatalf("scheduler.New: %v", err)
	}

	application := &App{
		appCore: appCore{
			Config: config.Config{
				Auth: config.AuthConfig{
					AutoGrantCapabilities: []string{"scheduler.create"},
				},
			},
			Logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		},
		appPlatform: appPlatform{
			Scheduler: engine,
		},
		appPlugins: appPlugins{
			grantRepository: &stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
		},
	}
	application.pluginLifecycle = newPluginLifecycleController(application)

	first, err := application.executeLocalAction(context.Background(), "weather", "req_sched_1", runtime.Action{
		Kind:               "scheduler.create",
		SchedulerTaskID:    "daily_report",
		SchedulerCron:      "0 8 * * *",
		SchedulerEventType: "scheduler.trigger",
		SchedulerPayload: map[string]any{
			"topic": "daily_report",
		},
	})
	if err != nil {
		t.Fatalf("first scheduler.create failed: %v", err)
	}
	if first["task_id"] != "daily_report" {
		t.Fatalf("unexpected task_id: %#v", first["task_id"])
	}
	if _, ok := first["next_run"].(string); !ok {
		t.Fatalf("expected next_run string, got %#v", first["next_run"])
	}

	second, err := application.executeLocalAction(context.Background(), "weather", "req_sched_2", runtime.Action{
		Kind:               "scheduler.create",
		SchedulerTaskID:    "daily_report",
		SchedulerCron:      "30 9 * * *",
		SchedulerEventType: "scheduler.trigger",
		SchedulerPayload: map[string]any{
			"topic": "daily_report_v2",
		},
	})
	if err != nil {
		t.Fatalf("second scheduler.create failed: %v", err)
	}
	if second["task_id"] != "daily_report" {
		t.Fatalf("unexpected second task_id: %#v", second["task_id"])
	}

	jobs := engine.Jobs()
	if len(jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1", len(jobs))
	}
	if jobs[0].JobID != "daily_report" || jobs[0].CronExpr != "30 9 * * *" {
		t.Fatalf("unexpected upserted job: %#v", jobs[0])
	}
}

func TestExecuteExposeWebhookRegistersGateway(t *testing.T) {
	t.Parallel()

	application := &App{
		appCore: appCore{
			Config: config.Config{
				Server: config.ServerConfig{
					Host: "127.0.0.1",
					Port: 8080,
				},
				Auth: config.AuthConfig{
					AutoGrantCapabilities: []string{"event.expose_webhook"},
				},
			},
			Logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		},
		appPlugins: appPlugins{
			webhooks:        newPluginWebhookRegistry(),
			grantRepository: &stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
		},
	}
	application.pluginLifecycle = newPluginLifecycleController(application)

	application.grantRepository = &stubLifecycleGrantRepository{
		grants: map[string][]plugins.PluginGrant{
			"repo-watcher": {{
				PluginID:   "repo-watcher",
				Capability: "event.expose_webhook",
				ScopeJSON:  `{"webhooks":[{"route":"github","auth_strategy":"hmac_sha256","header":"X-Hub-Signature-256","secret_ref":"webhook.github.secret","source_ips":["192.0.2.0/24"]}]}`,
			}},
		},
	}

	result, err := application.executeLocalAction(context.Background(), "repo-watcher", "req_webhook_1", runtime.Action{
		Kind:                   "event.expose_webhook",
		WebhookRoute:           "github",
		WebhookMethods:         []string{"POST"},
		WebhookAuthStrategy:    "hmac_sha256",
		WebhookHeader:          "X-Hub-Signature-256",
		WebhookSecretRef:       "webhook.github.secret",
		WebhookSignaturePrefix: "sha256=",
	})
	if err != nil {
		t.Fatalf("event.expose_webhook failed: %v", err)
	}
	if result["route"] != "github" {
		t.Fatalf("unexpected route result: %#v", result)
	}
	urlValue, _ := result["url"].(string)
	if urlValue != "http://127.0.0.1:8080/api/webhooks/repo-watcher/github" {
		t.Fatalf("unexpected webhook url: %#v", urlValue)
	}

	registration, ok := application.webhooks.Get("repo-watcher", "github")
	if !ok {
		t.Fatal("expected webhook registration to be stored")
	}
	if registration.AuthStrategy != "hmac_sha256" || registration.SecretRef != "webhook.github.secret" {
		t.Fatalf("unexpected webhook registration: %#v", registration)
	}
	if len(registration.SourceIPs) != 1 || registration.SourceIPs[0] != "192.0.2.0/24" {
		t.Fatalf("unexpected webhook source IPs: %#v", registration.SourceIPs)
	}
}

func TestExecuteStorageFileRoundTrip(t *testing.T) {
	t.Parallel()

	application := &App{
		appCore: appCore{
			Config: config.Config{
				Storage: config.StorageConfig{
					FileMaxBytes:    1024,
					PluginWorkDirMB: 1,
				},
			},
			Logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		},
		appPlugins: appPlugins{
			pluginFiles: pluginfile.NewService(filepath.Join(t.TempDir(), "plugins")),
			grantRepository: &stubLifecycleGrantRepository{
				grants: map[string][]plugins.PluginGrant{
					"scope-cache": {{
						PluginID:   "scope-cache",
						Capability: "storage.file",
						ScopeJSON:  `{"storage_roots":["plugin_data"]}`,
					}},
				},
			},
		},
	}
	application.pluginLifecycle = newPluginLifecycleController(application)

	if _, err := application.executeLocalAction(context.Background(), "scope-cache", "req_local_file_1", runtime.Action{
		Kind:             "storage.file",
		StorageOperation: "write",
		StorageRoot:      "plugin_data",
		StoragePath:      "cache/example.txt",
		StorageContent:   []byte("hello file"),
	}); err != nil {
		t.Fatalf("storage.file write failed: %v", err)
	}

	readResult, err := application.executeLocalAction(context.Background(), "scope-cache", "req_local_file_2", runtime.Action{
		Kind:             "storage.file",
		StorageOperation: "read",
		StorageRoot:      "plugin_data",
		StoragePath:      "cache/example.txt",
	})
	if err != nil {
		t.Fatalf("storage.file read failed: %v", err)
	}
	if got := readResult["content_text"]; got != "hello file" {
		t.Fatalf("unexpected text content: %#v", got)
	}

	if _, err := application.executeLocalAction(context.Background(), "scope-cache", "req_local_file_3", runtime.Action{
		Kind:             "storage.file",
		StorageOperation: "write",
		StorageRoot:      "plugin_data",
		StoragePath:      "cache/blob.bin",
		StorageContent:   []byte{0xff, 0x00, 0x01},
	}); err != nil {
		t.Fatalf("storage.file binary write failed: %v", err)
	}

	binaryResult, err := application.executeLocalAction(context.Background(), "scope-cache", "req_local_file_4", runtime.Action{
		Kind:             "storage.file",
		StorageOperation: "read",
		StorageRoot:      "plugin_data",
		StoragePath:      "cache/blob.bin",
	})
	if err != nil {
		t.Fatalf("storage.file binary read failed: %v", err)
	}
	if got := binaryResult["content_base64"]; got != base64.StdEncoding.EncodeToString([]byte{0xff, 0x00, 0x01}) {
		t.Fatalf("unexpected base64 content: %#v", got)
	}
}

func TestExecuteStorageFileRejectsMissingScope(t *testing.T) {
	t.Parallel()

	application := &App{
		appCore: appCore{
			Config: config.Config{},
			Logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		},
		appPlugins: appPlugins{
			pluginFiles: pluginfile.NewService(filepath.Join(t.TempDir(), "plugins")),
			grantRepository: &stubLifecycleGrantRepository{
				grants: map[string][]plugins.PluginGrant{
					"scope-cache": {{
						PluginID:   "scope-cache",
						Capability: "storage.file",
						ScopeJSON:  `{"storage_roots":[]}`,
					}},
				},
			},
		},
	}
	application.pluginLifecycle = newPluginLifecycleController(application)

	_, err := application.executeLocalAction(context.Background(), "scope-cache", "req_local_file_5", runtime.Action{
		Kind:             "storage.file",
		StorageOperation: "read",
		StorageRoot:      "plugin_data",
		StoragePath:      "cache/example.txt",
	})
	assertRuntimeErrorCode(t, err, "permission.scope_violation")
}

func TestExecuteRenderImageReturnsArtifact(t *testing.T) {
	t.Parallel()

	renderRoot := filepath.Join(t.TempDir(), "render")
	application := &App{
		appCore: appCore{
			Config: config.Config{
				Auth: config.AuthConfig{
					AutoGrantCapabilities: []string{"render.image"},
				},
			},
			Logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		},
		appPlugins: appPlugins{
			renderer:        newRenderService(renderRoot),
			grantRepository: &stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
		},
	}
	application.pluginLifecycle = newPluginLifecycleController(application)

	result, err := application.executeLocalAction(context.Background(), "help-menu", "req_render_1", runtime.Action{
		Kind:               "render.image",
		RenderTemplate:     "help.menu",
		RenderTheme:        "default",
		RenderOutput:       "png",
		RenderFallbackText: "帮助菜单暂时不可用。",
		RenderData: map[string]any{
			"title": "帮助菜单",
		},
	})
	if err != nil {
		t.Fatalf("render.image failed: %v", err)
	}
	if result["mime"] != "image/png" {
		t.Fatalf("unexpected render mime: %#v", result["mime"])
	}
	imagePath, ok := result["image_path"].(string)
	if !ok || imagePath == "" {
		t.Fatalf("unexpected render image path: %#v", result["image_path"])
	}
	parsed, err := url.Parse(imagePath)
	if err != nil || parsed.Scheme != "file" {
		t.Fatalf("unexpected file url %q: %v", imagePath, err)
	}
	if _, err := filepath.Abs(filepath.FromSlash(parsed.Path)); err != nil {
		t.Fatalf("unexpected render file path: %v", err)
	}
	if cacheKey, ok := result["cache_key"].(string); !ok || cacheKey == "" {
		t.Fatalf("unexpected cache key: %#v", result["cache_key"])
	}
}

func TestExecuteHTTPRequestUsesGrantedScopeAndReturnsText(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/data" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello http"))
	}))
	defer server.Close()

	application := &App{
		appCore: appCore{
			Config: config.Config{
				HTTP: config.HTTPConfig{
					TimeoutSeconds:    5,
					MaxRetries:        0,
					AllowPrivateHosts: []string{"127.0.0.1"},
				},
			},
			Logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		},
		appPlugins: appPlugins{
			grantRepository: &stubLifecycleGrantRepository{
				grants: map[string][]plugins.PluginGrant{
					"scope-cache": {{
						PluginID:   "scope-cache",
						Capability: "http.request",
						ScopeJSON:  `{"http_hosts":["127.0.0.1"]}`,
					}},
				},
			},
		},
	}
	application.pluginLifecycle = newPluginLifecycleController(application)

	result, err := application.executeLocalAction(context.Background(), "scope-cache", "req_http_1", runtime.Action{
		Kind:       "http.request",
		HTTPMethod: "GET",
		HTTPURL:    server.URL + "/v1/data",
	})
	if err != nil {
		t.Fatalf("http.request failed: %v", err)
	}
	if got := result["status_code"]; got != http.StatusOK {
		t.Fatalf("unexpected status_code: %#v", got)
	}
	if got := result["body_text"]; got != "hello http" {
		t.Fatalf("unexpected body_text: %#v", got)
	}
}

func TestExecuteHTTPRequestRejectsPrivateHostWithoutAllowlist(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	application := &App{
		appCore: appCore{
			Config: config.Config{
				HTTP: config.HTTPConfig{
					TimeoutSeconds: 5,
					MaxRetries:     0,
				},
			},
			Logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		},
		appPlugins: appPlugins{
			grantRepository: &stubLifecycleGrantRepository{
				grants: map[string][]plugins.PluginGrant{
					"scope-cache": {{
						PluginID:   "scope-cache",
						Capability: "http.request",
						ScopeJSON:  `{"http_hosts":["127.0.0.1"]}`,
					}},
				},
			},
		},
	}
	application.pluginLifecycle = newPluginLifecycleController(application)

	_, err := application.executeLocalAction(context.Background(), "scope-cache", "req_http_2", runtime.Action{
		Kind:       "http.request",
		HTTPMethod: "GET",
		HTTPURL:    server.URL + "/v1/data",
	})
	assertRuntimeErrorCode(t, err, "permission.scope_violation")
}

func assertRuntimeErrorCode(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected runtime error %q, got nil", want)
	}

	var runtimeErr *runtime.Error
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("expected *runtime.Error, got %T", err)
	}
	if runtimeErr.Code != want {
		t.Fatalf("unexpected runtime error code: got %q want %q", runtimeErr.Code, want)
	}
}
