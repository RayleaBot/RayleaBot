package lifecycle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

const runtimeDependenciesMarker = ".rayleabot-dependencies.json"

type dependencyInstaller struct {
	python func(context.Context, string, string, []string) error
	node   func(context.Context, string, string, []string, bool) error
}

type dependencyMarker struct {
	Fingerprint string `json:"fingerprint"`
}

func prepareBuiltinDependencies(ctx context.Context, repoRoot string, snapshot plugins.Snapshot) error {
	return prepareBuiltinDependenciesWith(ctx, repoRoot, snapshot, dependencyInstaller{
		python: preparePythonEnvironment,
		node:   prepareNodeEnvironment,
	})
}

func prepareBuiltinDependenciesWith(ctx context.Context, repoRoot string, snapshot plugins.Snapshot, installer dependencyInstaller) error {
	if snapshot.SourceRoot != "plugins/builtin" {
		return nil
	}

	pluginDir := snapshot.PackageRootPath
	if pluginDir == "" {
		return errors.New("built-in plugin package root is missing")
	}

	fingerprint, err := runtimeDependenciesFingerprint(snapshot)
	if err != nil {
		return err
	}

	switch snapshot.Runtime {
	case "python":
		if len(snapshot.PythonDependencies) == 0 {
			return nil
		}
		venvDir := filepath.Join(pluginDir, ".venv")
		markerPath := filepath.Join(venvDir, runtimeDependenciesMarker)
		if dependencyMarkerMatches(markerPath, fingerprint) {
			if _, err := virtualenvPythonExecutable(venvDir); err == nil {
				return nil
			}
		}
		if installer.python == nil {
			return errors.New("Python dependency installer is unavailable")
		}
		if err := installer.python(ctx, repoRoot, pluginDir, snapshot.PythonDependencies); err != nil {
			return fmt.Errorf("install Python dependencies from package index: %w", err)
		}
		return writeDependencyMarker(markerPath, fingerprint)
	case "nodejs":
		if len(snapshot.NodeDependencies) == 0 && !snapshot.RequireInstallScripts {
			return nil
		}
		nodeModulesDir := filepath.Join(pluginDir, "node_modules")
		markerPath := filepath.Join(nodeModulesDir, runtimeDependenciesMarker)
		if dependencyMarkerMatches(markerPath, fingerprint) {
			if info, err := os.Stat(nodeModulesDir); err == nil && info.IsDir() {
				return nil
			}
		}
		if installer.node == nil {
			return errors.New("Node.js dependency installer is unavailable")
		}
		if err := installer.node(ctx, repoRoot, pluginDir, snapshot.NodeDependencies, snapshot.RequireInstallScripts); err != nil {
			return fmt.Errorf("install Node.js dependencies from package registry: %w", err)
		}
		return writeDependencyMarker(markerPath, fingerprint)
	default:
		return nil
	}
}

func runtimeDependenciesFingerprint(snapshot plugins.Snapshot) (string, error) {
	pythonDependencies := append([]string(nil), snapshot.PythonDependencies...)
	nodeDependencies := append([]string(nil), snapshot.NodeDependencies...)
	sort.Strings(pythonDependencies)
	sort.Strings(nodeDependencies)

	packageFiles := map[string]string{}
	if snapshot.Runtime == "nodejs" {
		for _, name := range []string{"package.json", "package-lock.json", "npm-shrinkwrap.json"} {
			content, err := os.ReadFile(filepath.Join(snapshot.PackageRootPath, name))
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				return "", fmt.Errorf("read %s for dependency fingerprint: %w", name, err)
			}
			digest := sha256.Sum256(content)
			packageFiles[name] = hex.EncodeToString(digest[:])
		}
	}

	payload, err := json.Marshal(struct {
		Runtime               string            `json:"runtime"`
		PythonDependencies    []string          `json:"python_dependencies,omitempty"`
		NodeDependencies      []string          `json:"node_dependencies,omitempty"`
		RequireInstallScripts bool              `json:"require_install_scripts,omitempty"`
		PackageFiles          map[string]string `json:"package_files,omitempty"`
	}{
		Runtime:               snapshot.Runtime,
		PythonDependencies:    pythonDependencies,
		NodeDependencies:      nodeDependencies,
		RequireInstallScripts: snapshot.RequireInstallScripts,
		PackageFiles:          packageFiles,
	})
	if err != nil {
		return "", fmt.Errorf("encode dependency fingerprint: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func dependencyMarkerMatches(path, fingerprint string) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var marker dependencyMarker
	return json.Unmarshal(content, &marker) == nil && marker.Fingerprint == fingerprint
}

func writeDependencyMarker(path, fingerprint string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dependency marker directory: %w", err)
	}
	content, err := json.Marshal(dependencyMarker{Fingerprint: fingerprint})
	if err != nil {
		return fmt.Errorf("encode dependency marker: %w", err)
	}
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, append(content, '\n'), 0o644); err != nil {
		return fmt.Errorf("write dependency marker: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("publish dependency marker: %w", err)
	}
	return nil
}
