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
	"gopkg.in/yaml.v3"
)

func TestCreateBuildsSchemaValidArchiveWithPluginBusinessData(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, "config", "user.yaml")
	databasePath := filepath.Join(repoRoot, "data", "rayleabot.db")
	const sparseConfig = "database:\n  path: data/rayleabot.db\n"
	writeArchiveFixture(t, configPath, sparseConfig)
	writeArchiveFixture(t, filepath.Join(repoRoot, "data", "plugins", "weather", "state.json"), `{"cursor":42}`)

	store, err := storage.Open(databasePath)
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	defer func(release func() error) { _ = release() }(store.Close)

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
	if original, err := os.ReadFile(configPath); err != nil || string(original) != sparseConfig {
		t.Fatalf("backup changed the source config: %q %v", original, err)
	}

	reader, err := zip.OpenReader(result.ArchivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer func(release func() error) { _ = release() }(reader.Close)
	names := make(map[string]bool)
	for _, entry := range reader.File {
		names[entry.Name] = true
		if entry.Name == "config/user.yaml" {
			stream, err := entry.Open()
			if err != nil {
				t.Fatal(err)
			}
			var document map[string]any
			err = yaml.NewDecoder(stream).Decode(&document)
			_ = stream.Close()
			if err != nil || document["schema_version"] != result.Manifest.ConfigSchemaVersion {
				t.Fatalf("archived effective config version differs from manifest: %#v %v", document, err)
			}
			if _, ok := document["server"].(map[string]any)["port"].(int); !ok {
				t.Fatal("archived schema default integer lost its YAML type")
			}
		}
		if entry.Name == "backup-manifest.json" {
			stream, err := entry.Open()
			if err != nil {
				t.Fatalf("open manifest entry: %v", err)
			}
			var manifest recovery.BackupManifest
			if err := json.NewDecoder(stream).Decode(&manifest); err != nil {
				_ = stream.Close()
				t.Fatalf("decode manifest entry: %v", err)
			}
			_ = stream.Close()
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

func TestCreateConfigurationBackupRecordsAbsentDatabase(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, "config", "user.yaml")
	writeArchiveFixture(t, configPath, "server:\n  host: 127.0.0.1\n")
	result, err := Create(t.Context(), Options{
		RepoRoot: repoRoot, ConfigPath: configPath, Consistency: "offline",
		CreateSnapshot: func(context.Context, string) (string, error) { return "", os.ErrNotExist },
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Manifest.DBSchemaVersion != "absent" || result.Manifest.ConfigSchemaVersion != "4" {
		t.Fatalf("configuration-only backup metadata = %#v", result.Manifest)
	}
	for _, directory := range result.Manifest.Directories {
		if directory.Label == "database" {
			t.Fatal("backup advertised a missing database snapshot")
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
