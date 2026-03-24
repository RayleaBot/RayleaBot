package app

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDatabasePathUsesTopLevelDataRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	configPath := filepath.Join(root, "config", "user.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}

	resolved, err := resolveDatabasePath(configPath, filepath.Join("data", "rayleabot.db"))
	if err != nil {
		t.Fatalf("resolve database path: %v", err)
	}

	expected := filepath.Join(root, "data", "rayleabot.db")
	if resolved != expected {
		t.Fatalf("resolved database path = %s, want %s", resolved, expected)
	}
}

func TestMigrateLegacyDataRootMovesManagedEntriesToTopLevelData(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	configPath := filepath.Join(root, "config", "user.yaml")
	legacyDataRoot := filepath.Join(root, "config", "data")
	canonicalDataRoot := filepath.Join(root, "data")

	if err := os.MkdirAll(filepath.Join(legacyDataRoot, "plugins"), 0o755); err != nil {
		t.Fatalf("create legacy plugins dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(legacyDataRoot, "render"), 0o755); err != nil {
		t.Fatalf("create legacy render dir: %v", err)
	}
	writeTestFile(t, filepath.Join(legacyDataRoot, "rayleabot.db"), "db")
	writeTestFile(t, filepath.Join(legacyDataRoot, "rayleabot.db-wal"), "wal")
	writeTestFile(t, filepath.Join(legacyDataRoot, "rayleabot.db-shm"), "shm")
	writeTestFile(t, filepath.Join(legacyDataRoot, "plugins", "plugin.txt"), "plugin")
	writeTestFile(t, filepath.Join(legacyDataRoot, "render", "asset.txt"), "render")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := migrateLegacyDataRoot(logger, configPath, filepath.Join("data", "rayleabot.db")); err != nil {
		t.Fatalf("migrate legacy data root: %v", err)
	}

	for _, path := range []string{
		filepath.Join(canonicalDataRoot, "rayleabot.db"),
		filepath.Join(canonicalDataRoot, "rayleabot.db-wal"),
		filepath.Join(canonicalDataRoot, "rayleabot.db-shm"),
		filepath.Join(canonicalDataRoot, "plugins", "plugin.txt"),
		filepath.Join(canonicalDataRoot, "render", "asset.txt"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected migrated entry %s: %v", path, err)
		}
	}

	if _, err := os.Stat(legacyDataRoot); !os.IsNotExist(err) {
		t.Fatalf("legacy data root should be removed after migration, got err=%v", err)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
