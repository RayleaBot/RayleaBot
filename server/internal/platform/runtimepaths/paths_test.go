package runtimepaths

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

	resolved, err := ResolveDatabasePath(configPath, filepath.Join("data", "rayleabot.db"))
	if err != nil {
		t.Fatalf("resolve database path: %v", err)
	}

	expected := filepath.Join(root, "data", "rayleabot.db")
	if resolved != expected {
		t.Fatalf("resolved database path = %s, want %s", resolved, expected)
	}
}

func TestResolveConfigLifecycleLockPathFollowsConfigFile(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
	resolved, err := ResolveConfigLifecycleLockPath(configPath)
	if err != nil {
		t.Fatalf("resolve config lifecycle lock path: %v", err)
	}
	want := configPath + ".runtime.lock"
	if resolved != want {
		t.Fatalf("resolved lock path = %s, want %s", resolved, want)
	}
}
