package backup

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func TestCreateBuildsSchemaValidArchiveWithPluginBusinessData(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, "config", "user.yaml")
	databasePath := filepath.Join(repoRoot, "data", "rayleabot.db")
	writeArchiveFixture(t, configPath, "database:\n  path: data/rayleabot.db\n")
	writeArchiveFixture(t, filepath.Join(repoRoot, "data", "plugins", "weather", "state.json"), `{"cursor":42}`)

	store, err := storage.Open(databasePath)
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	defer store.Close()

	result, err := Create(t.Context(), Options{
		RepoRoot:       repoRoot,
		ConfigPath:     configPath,
		DatabasePath:   databasePath,
		Consistency:    "online",
		CreateSnapshot: func(ctx context.Context, _ string) (string, error) { return store.CreateSnapshot(ctx) },
		Now:            func() time.Time { return time.Date(2026, 8, 23, 8, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if err := recovery.ValidateBackupManifest(result.Manifest); err != nil {
		t.Fatalf("manifest validation failed: %v", err)
	}

	reader, err := zip.OpenReader(result.ArchivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer reader.Close()
	names := make(map[string]bool)
	for _, entry := range reader.File {
		names[entry.Name] = true
		if entry.Name == "backup-manifest.json" {
			stream, err := entry.Open()
			if err != nil {
				t.Fatalf("open manifest entry: %v", err)
			}
			var manifest recovery.BackupManifest
			if err := json.NewDecoder(stream).Decode(&manifest); err != nil {
				stream.Close()
				t.Fatalf("decode manifest entry: %v", err)
			}
			stream.Close()
			if err := recovery.ValidateBackupManifest(manifest); err != nil {
				t.Fatalf("archived manifest validation failed: %v", err)
			}
		}
	}
	for _, required := range []string{
		"backup-manifest.json",
		"config/user.yaml",
		"data/rayleabot.db",
		"data/plugins/weather/state.json",
	} {
		if !names[required] {
			t.Fatalf("archive missing %s: %#v", required, names)
		}
	}
}

func writeArchiveFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
