package plugins

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"rayleabot/server/internal/storage"
)

func TestSQLiteRepositoryRoundTripsDesiredStates(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("Open sqlite store failed: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close sqlite store: %v", err)
		}
	})

	repo, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("NewSQLiteRepository failed: %v", err)
	}

	if err := repo.SaveDesiredState(context.Background(), "hello-node", "enabled", time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("SaveDesiredState hello-node failed: %v", err)
	}
	if err := repo.SaveDesiredState(context.Background(), "hello-python", "disabled", time.Date(2026, 3, 20, 12, 5, 0, 0, time.UTC)); err != nil {
		t.Fatalf("SaveDesiredState hello-python failed: %v", err)
	}

	states, err := repo.LoadDesiredStates(context.Background())
	if err != nil {
		t.Fatalf("LoadDesiredStates failed: %v", err)
	}

	if states["hello-node"] != "enabled" {
		t.Fatalf("hello-node desired_state = %q, want enabled", states["hello-node"])
	}
	if states["hello-python"] != "disabled" {
		t.Fatalf("hello-python desired_state = %q, want disabled", states["hello-python"])
	}
}

func TestCatalogApplyDesiredStatesOverridesInstalledEntriesOnly(t *testing.T) {
	t.Parallel()

	catalog := NewCatalog([]Snapshot{
		{
			PluginID:          "hello-node",
			RegistrationState: "installed",
			DesiredState:      "disabled",
			RuntimeState:      "stopped",
		},
		{
			PluginID:          "removed-plugin",
			RegistrationState: "removed",
			DesiredState:      "disabled",
			RuntimeState:      "stopped",
		},
	})

	catalog.ApplyDesiredStates(map[string]string{
		"hello-node":     "enabled",
		"removed-plugin": "enabled",
		"missing-plugin": "enabled",
		"hello-python":   "paused",
	})

	helloNode, _ := catalog.Get("hello-node")
	if helloNode.DesiredState != "enabled" {
		t.Fatalf("hello-node desired_state = %q, want enabled", helloNode.DesiredState)
	}

	removed, _ := catalog.Get("removed-plugin")
	if removed.DesiredState != "disabled" {
		t.Fatalf("removed-plugin desired_state = %q, want disabled", removed.DesiredState)
	}
}
