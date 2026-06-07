package app

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/schemaassets"
)

type pluginDiscoverySpec struct {
	repoRoot         string
	pluginSchemaPath string
	roots            []plugins.ScanRoot
}

func resolveDatabasePath(configPath, databasePath string) (string, error) {
	if filepath.IsAbs(databasePath) {
		return filepath.Clean(databasePath), nil
	}

	repoRoot, err := resolveRuntimeRoot(configPath)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.Abs(filepath.Join(repoRoot, databasePath))
	if err != nil {
		return "", fmt.Errorf("resolve database path %s: %w", databasePath, err)
	}

	return resolved, nil
}

func resolveRuntimeRoot(configPath string) (string, error) {
	absoluteConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return "", fmt.Errorf("resolve runtime root from %s: %w", configPath, err)
	}
	return recovery.RepoRootFromConfigPath(absoluteConfigPath), nil
}

func resolvePluginDiscovery(options Options) (pluginDiscoverySpec, error) {
	if len(options.PluginRoots) > 0 || options.PluginRepoRoot != "" || options.PluginSchemaPath != "" {
		if options.PluginRepoRoot == "" || options.PluginSchemaPath == "" || len(options.PluginRoots) == 0 {
			return pluginDiscoverySpec{}, fmt.Errorf("plugin discovery override requires repo root, schema path, and roots")
		}
		return pluginDiscoverySpec{
			repoRoot:         options.PluginRepoRoot,
			pluginSchemaPath: options.PluginSchemaPath,
			roots:            append([]plugins.ScanRoot(nil), options.PluginRoots...),
		}, nil
	}

	repoRoot, pluginSchemaPath, roots, err := pluginDiscoveryContext(options.ConfigPath)
	if err != nil {
		return pluginDiscoverySpec{}, err
	}
	return pluginDiscoverySpec{
		repoRoot:         repoRoot,
		pluginSchemaPath: pluginSchemaPath,
		roots:            roots,
	}, nil
}

func pluginDiscoveryContext(configPath string) (string, string, []plugins.ScanRoot, error) {
	repoRoot, err := resolveRuntimeRoot(configPath)
	if err != nil {
		return "", "", nil, err
	}
	pluginSchemaPath := schemaassets.PluginInfoSchemaID

	roots := []plugins.ScanRoot{
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

func cleanupOrphanedInstallDirs(logger *slog.Logger, roots []plugins.ScanRoot) {
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
