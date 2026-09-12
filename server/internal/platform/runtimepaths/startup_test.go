package runtimepaths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigPathUsesInstallationRoot(t *testing.T) {
	t.Parallel()
	sourceMarkers := []string{"source/server/go.mod", "source/launcher/package.json"}
	releaseMarkers := []string{"release/build_info.json", "release/.deps/manifest.json"}
	tests := []struct {
		name       string
		files      []string
		workingDir string
		executable string
		wantRoot   string
	}{
		{"source root", sourceMarkers, "source", "go-build/server", "source"},
		{"source server directory", sourceMarkers, "source/server", "go-build/server", "source"},
		{"source nested directory", sourceMarkers, "source/server/cmd/raylea-server", "go-build/server", "source"},
		{"source build output", sourceMarkers, "elsewhere", "source/server/dist/raylea-server", "source"},
		{"release executable", releaseMarkers, "elsewhere", "release/raylea-server", "release"},
		{"release nested directory", releaseMarkers, "release/cache/nested", "go-build/server", "release"},
		{"executable installation takes priority", append(append([]string{}, sourceMarkers...), releaseMarkers...), "source/server", "release/raylea-server", "release"},
		{"nearest installation", append(append([]string{}, sourceMarkers...), "source/nested/build_info.json", "source/nested/.deps/manifest.json"), "source/nested/cache", "go-build/server", "source/nested"},
		{"misplaced config does not select a root", append(append([]string{}, sourceMarkers...), "source/server/config/user.yaml"), "source/server", "go-build/server", "source"},
		{"standalone directory", nil, "runtime", "go-build/server", "runtime"},
		{"partial markers do not select a root", []string{"source/server/go.mod", "source/build_info.json"}, "source/server", "go-build/server", "source/server"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			for _, name := range tt.files {
				path := filepath.Join(root, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("fixture\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			workingDir := filepath.Join(root, filepath.FromSlash(tt.workingDir))
			if err := os.MkdirAll(workingDir, 0o755); err != nil {
				t.Fatal(err)
			}
			configPath := defaultConfigPath(workingDir, filepath.Join(root, filepath.FromSlash(tt.executable)))
			want := filepath.Join(root, filepath.FromSlash(tt.wantRoot), "config", "user.yaml")
			if configPath != want {
				t.Fatalf("configuration path = %s, want %s", configPath, want)
			}
			databasePath, err := ResolveDatabasePath(configPath, "data/rayleabot.db")
			if err != nil {
				t.Fatal(err)
			}
			if want := filepath.Join(root, filepath.FromSlash(tt.wantRoot), "data", "rayleabot.db"); databasePath != want {
				t.Fatalf("database path = %s, want %s", databasePath, want)
			}
		})
	}
}

func TestResolveStartupConfigPathHonorsExplicitPath(t *testing.T) {
	t.Parallel()
	for _, path := range []string{DefaultConfigPath, filepath.Join("..", "custom", "config", "user.yaml"), filepath.Join(t.TempDir(), "config", "user.yaml")} {
		got, err := ResolveStartupConfigPath(path, true)
		if err != nil {
			t.Fatal(err)
		}
		want, err := filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("explicit configuration path = %s, want %s", got, want)
		}
	}
	if _, err := ResolveStartupConfigPath("", true); err == nil {
		t.Fatal("empty explicit configuration path was accepted")
	}
}
