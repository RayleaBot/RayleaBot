package app

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"testing"

	"rayleabot/server/internal/pluginconfig"
	"rayleabot/server/internal/plugins"
	"rayleabot/server/internal/storage"
)

func TestSeedPluginDefaultConfigSeedsOnlyOnce(t *testing.T) {
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
			Logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		},
		appPlugins: appPlugins{
			pluginConfig: repo,
		},
	}
	controller := newPluginLifecycleController(application)

	snapshot := plugins.Snapshot{
		PluginID: "weather",
		DefaultConfig: map[string]any{
			"default_city": "北京",
			"unit":         "celsius",
		},
	}
	if err := controller.seedPluginDefaultConfig(context.Background(), snapshot); err != nil {
		t.Fatalf("seedPluginDefaultConfig first call: %v", err)
	}

	if _, err := repo.Write(context.Background(), "weather", map[string]any{
		"default_city": "上海",
	}); err != nil {
		t.Fatalf("repo.Write: %v", err)
	}

	if err := controller.seedPluginDefaultConfig(context.Background(), snapshot); err != nil {
		t.Fatalf("seedPluginDefaultConfig second call: %v", err)
	}

	values, err := repo.Read(context.Background(), "weather", []string{"default_city", "unit"})
	if err != nil {
		t.Fatalf("repo.Read: %v", err)
	}
	if values["default_city"] != "上海" {
		t.Fatalf("expected existing config to be preserved, got %#v", values)
	}
	if values["unit"] != "celsius" {
		t.Fatalf("expected default unit to be preserved, got %#v", values)
	}
}
