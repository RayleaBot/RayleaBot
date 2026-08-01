package lifecycle

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestPrepareBuiltinDependenciesInstallsPythonRuntimeOnce(t *testing.T) {
	pluginDir := t.TempDir()
	installCount := 0
	snapshot := plugins.Snapshot{
		Runtime:            "python",
		SourceRoot:         "plugins/builtin",
		PackageRootPath:    pluginDir,
		PythonDependencies: []string{"rayleabot-plugin-runtime==0.1.0"},
	}
	installer := dependencyInstaller{
		python: func(_ context.Context, _, target string, dependencies []string) error {
			installCount++
			if len(dependencies) != 1 || dependencies[0] != "rayleabot-plugin-runtime==0.1.0" {
				t.Fatalf("unexpected dependencies: %v", dependencies)
			}
			pythonPath := filepath.Join(target, ".venv", "Scripts", "python.exe")
			if err := os.MkdirAll(filepath.Dir(pythonPath), 0o755); err != nil {
				return err
			}
			return os.WriteFile(pythonPath, []byte("python"), 0o755)
		},
	}

	for range 2 {
		if err := prepareBuiltinDependenciesWith(context.Background(), "repo", snapshot, installer); err != nil {
			t.Fatalf("prepare dependencies: %v", err)
		}
	}
	if installCount != 1 {
		t.Fatalf("install count = %d, want 1", installCount)
	}
}

func TestPrepareBuiltinDependenciesRefreshesChangedNodeDependency(t *testing.T) {
	pluginDir := t.TempDir()
	installCount := 0
	installer := dependencyInstaller{
		node: func(_ context.Context, _, target string, _ []string, _ bool) error {
			installCount++
			return os.MkdirAll(filepath.Join(target, "node_modules"), 0o755)
		},
	}
	snapshot := plugins.Snapshot{
		Runtime:          "nodejs",
		SourceRoot:       "plugins/builtin",
		PackageRootPath:  pluginDir,
		NodeDependencies: []string{"@rayleabot/plugin-runtime@0.1.0"},
	}
	if err := prepareBuiltinDependenciesWith(context.Background(), "repo", snapshot, installer); err != nil {
		t.Fatalf("prepare initial dependencies: %v", err)
	}
	snapshot.NodeDependencies = []string{"@rayleabot/plugin-runtime@0.2.0"}
	if err := prepareBuiltinDependenciesWith(context.Background(), "repo", snapshot, installer); err != nil {
		t.Fatalf("prepare changed dependencies: %v", err)
	}
	if installCount != 2 {
		t.Fatalf("install count = %d, want 2", installCount)
	}
}

func TestPrepareBuiltinDependenciesDoesNotInstallThirdPartyPackages(t *testing.T) {
	called := false
	err := prepareBuiltinDependenciesWith(context.Background(), "repo", plugins.Snapshot{
		Runtime:            "python",
		SourceRoot:         "plugins/installed",
		PackageRootPath:    t.TempDir(),
		PythonDependencies: []string{"rayleabot-plugin-runtime==0.1.0"},
	}, dependencyInstaller{
		python: func(context.Context, string, string, []string) error {
			called = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("prepare dependencies: %v", err)
	}
	if called {
		t.Fatal("third-party dependencies must remain part of the explicit install flow")
	}
}
