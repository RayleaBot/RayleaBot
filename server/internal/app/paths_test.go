package app

import (
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
