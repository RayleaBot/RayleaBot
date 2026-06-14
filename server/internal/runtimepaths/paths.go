package runtimepaths

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	plugindiscovery "github.com/RayleaBot/RayleaBot/server/internal/plugins/discovery"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/schemaassets"
)

type PluginDiscoverySpec struct {
	RepoRoot         string
	PluginSchemaPath string
	Roots            []plugindiscovery.ScanRoot
}

func ResolveDatabasePath(configPath, databasePath string) (string, error) {
	if filepath.IsAbs(databasePath) {
		return filepath.Clean(databasePath), nil
	}

	repoRoot, err := ResolveRuntimeRoot(configPath)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.Abs(filepath.Join(repoRoot, databasePath))
	if err != nil {
		return "", fmt.Errorf("resolve database path %s: %w", databasePath, err)
	}

	return resolved, nil
}

func ResolveRuntimeRoot(configPath string) (string, error) {
	absoluteConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return "", fmt.Errorf("resolve runtime root from %s: %w", configPath, err)
	}
	return recovery.RepoRootFromConfigPath(absoluteConfigPath), nil
}

type PluginDiscoveryOptions struct {
	ConfigPath       string
	PluginRepoRoot   string
	PluginSchemaPath string
	PluginRoots      []plugindiscovery.ScanRoot
}

func ResolvePluginDiscovery(options PluginDiscoveryOptions) (PluginDiscoverySpec, error) {
	if len(options.PluginRoots) > 0 || options.PluginRepoRoot != "" || options.PluginSchemaPath != "" {
		if options.PluginRepoRoot == "" || options.PluginSchemaPath == "" || len(options.PluginRoots) == 0 {
			return PluginDiscoverySpec{}, fmt.Errorf("plugin discovery override requires repo root, schema path, and roots")
		}
		return PluginDiscoverySpec{
			RepoRoot:         options.PluginRepoRoot,
			PluginSchemaPath: options.PluginSchemaPath,
			Roots:            append([]plugindiscovery.ScanRoot(nil), options.PluginRoots...),
		}, nil
	}

	repoRoot, pluginSchemaPath, roots, err := PluginDiscoveryContext(options.ConfigPath)
	if err != nil {
		return PluginDiscoverySpec{}, err
	}
	return PluginDiscoverySpec{
		RepoRoot:         repoRoot,
		PluginSchemaPath: pluginSchemaPath,
		Roots:            roots,
	}, nil
}

func PluginDiscoveryContext(configPath string) (string, string, []plugindiscovery.ScanRoot, error) {
	repoRoot, err := ResolveRuntimeRoot(configPath)
	if err != nil {
		return "", "", nil, err
	}
	pluginSchemaPath := schemaassets.PluginInfoSchemaID

	roots := []plugindiscovery.ScanRoot{
		{
			Label: "plugins/builtin",
			Path:  filepath.Join(repoRoot, "plugins", "builtin"),
		},
		{
			Label: "plugins/installed",
			Path:  filepath.Join(repoRoot, "plugins", "installed"),
		},
	}

	return repoRoot, pluginSchemaPath, roots, nil
}

func CleanupOrphanedInstallDirs(logger *slog.Logger, roots []plugindiscovery.ScanRoot) {
	for _, root := range roots {
		if root.Label != "plugins/installed" {
			continue
		}
		entries, err := os.ReadDir(root.Path)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			if len(name) > len(".plugin-install-") && name[:len(".plugin-install-")] == ".plugin-install-" {
				orphanPath := filepath.Join(root.Path, name)
				if err := os.RemoveAll(orphanPath); err != nil {
					logger.Warn("failed to clean up orphaned install directory",
						"component", "app",
						"path", orphanPath,
						"err", err.Error(),
					)
				} else {
					logger.Info("cleaned up orphaned install directory",
						"component", "app",
						"path", orphanPath,
					)
				}
			}
		}
	}
}
