package lifecycle

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginstore"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func TestSeedPluginDefaultConfigAddsMissingDefaultsWithoutOverwriting(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer store.Close()

	repo, err := pluginstore.NewConfigSQLiteRepository(store)
	if err != nil {
		t.Fatalf("NewSQLiteRepository: %v", err)
	}

	application := newTestAppState(config.Config{}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	controller := NewController(Deps{
		CurrentConfig: application.state.CurrentConfig,
		Logger:        application.state.Logger,
		PluginConfig:  repo,
	})

	snapshot := plugins.Snapshot{
		PluginID: "weather",
		DefaultConfig: map[string]any{
			"default_city": "Beijing",
			"unit":         "celsius",
		},
	}
	if err := controller.seedPluginDefaultConfig(context.Background(), snapshot); err != nil {
		t.Fatalf("seedPluginDefaultConfig first call: %v", err)
	}

	if _, err := repo.Write(context.Background(), "weather", map[string]any{
		"default_city": "Shanghai",
	}); err != nil {
		t.Fatalf("repo.Write: %v", err)
	}

	snapshot.DefaultConfig["timeout"] = 15
	if err := controller.seedPluginDefaultConfig(context.Background(), snapshot); err != nil {
		t.Fatalf("seedPluginDefaultConfig second call: %v", err)
	}

	values, err := repo.Read(context.Background(), "weather", []string{"default_city", "unit", "timeout"})
	if err != nil {
		t.Fatalf("repo.Read: %v", err)
	}
	if values["default_city"] != "Shanghai" {
		t.Fatalf("expected existing config to be preserved, got %#v", values)
	}
	if values["unit"] != "celsius" {
		t.Fatalf("expected default unit to be preserved, got %#v", values)
	}
	if values["timeout"] != float64(15) {
		t.Fatalf("expected newly added default to be seeded, got %#v", values)
	}
}
