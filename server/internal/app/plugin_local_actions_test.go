package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"

	"rayleabot/server/internal/config"
	"rayleabot/server/internal/pluginkv"
	"rayleabot/server/internal/plugins"
	"rayleabot/server/internal/runtime"
	"rayleabot/server/internal/storage"
)

func TestExecuteLocalActionRejectsMissingCapability(t *testing.T) {
	t.Parallel()

	application := &App{
		Config: config.Config{},
		Logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		grantRepository: &stubLifecycleGrantRepository{
			grants: map[string][]plugins.PluginGrant{},
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
		Config: config.Config{
			Auth: config.AuthConfig{
				AutoGrantCapabilities: []string{"logger.write"},
			},
			Logging: config.LoggingConfig{
				RateLimitPerPlugin: "1/1h",
			},
		},
		Logger:           slog.New(slog.NewJSONHandler(buffer, nil)),
		pluginLogLimiter: newPluginLogLimiter(config.Config{Logging: config.LoggingConfig{RateLimitPerPlugin: "1/1h"}}),
		redactText: func(text string) string {
			return text
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
		Config: config.Config{
			Storage: config.StorageConfig{
				KVValueMaxBytes: 1024,
				KVTotalLimitMB:  1,
			},
		},
		Logger:   slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		pluginKV: repo,
		grantRepository: &stubLifecycleGrantRepository{
			grants: map[string][]plugins.PluginGrant{
				"notice-logger": {{
					PluginID:   "notice-logger",
					Capability: "storage.kv",
				}},
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
