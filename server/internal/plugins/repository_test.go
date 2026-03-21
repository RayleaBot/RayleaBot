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

func TestSQLiteRepositorySavesPackageMetadata(t *testing.T) {
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

	installedAt := time.Date(2026, 3, 21, 11, 0, 0, 0, time.UTC)
	err = repo.SavePackageMetadata(context.Background(), PackageMetadata{
		PluginID:     "hello-node",
		SourceType:   "local_directory",
		SourceRef:    "C:/plugins/hello-node",
		Version:      "0.2.0",
		ManifestHash: "manifest-sha",
		PackageHash:  "package-sha",
		InstalledAt:  installedAt,
	})
	if err != nil {
		t.Fatalf("SavePackageMetadata failed: %v", err)
	}

	var (
		sourceType   string
		sourceRef    string
		version      string
		manifestHash string
		packageHash  string
		recordedAt   string
	)
	if err := store.Read.QueryRow(
		`SELECT source_type, source_ref, version, manifest_hash, package_hash, installed_at
		   FROM plugin_packages
		  WHERE plugin_id = ?`,
		"hello-node",
	).Scan(&sourceType, &sourceRef, &version, &manifestHash, &packageHash, &recordedAt); err != nil {
		t.Fatalf("query plugin_packages row: %v", err)
	}

	if sourceType != "local_directory" {
		t.Fatalf("source_type = %q, want local_directory", sourceType)
	}
	if sourceRef != "C:/plugins/hello-node" {
		t.Fatalf("source_ref = %q, want C:/plugins/hello-node", sourceRef)
	}
	if version != "0.2.0" {
		t.Fatalf("version = %q, want 0.2.0", version)
	}
	if manifestHash != "manifest-sha" || packageHash != "package-sha" {
		t.Fatalf("unexpected hashes: manifest=%q package=%q", manifestHash, packageHash)
	}
	if recordedAt != installedAt.Format(time.RFC3339Nano) {
		t.Fatalf("installed_at = %q, want %q", recordedAt, installedAt.Format(time.RFC3339Nano))
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
