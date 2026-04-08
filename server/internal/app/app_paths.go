package app

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
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

func resolveLegacyDatabasePath(configPath, databasePath string) (string, error) {
	if filepath.IsAbs(databasePath) {
		return filepath.Clean(databasePath), nil
	}

	configDir := filepath.Dir(configPath)
	resolved, err := filepath.Abs(filepath.Join(configDir, databasePath))
	if err != nil {
		return "", fmt.Errorf("resolve legacy database path %s: %w", databasePath, err)
	}

	return resolved, nil
}

func migrateLegacyDataRoot(logger *slog.Logger, configPath, databasePath string) error {
	if filepath.IsAbs(databasePath) {
		return nil
	}

	canonicalDatabasePath, err := resolveDatabasePath(configPath, databasePath)
	if err != nil {
		return err
	}
	legacyDatabasePath, err := resolveLegacyDatabasePath(configPath, databasePath)
	if err != nil {
		return err
	}
	if canonicalDatabasePath == legacyDatabasePath {
		return nil
	}

	canonicalDataRoot := filepath.Dir(canonicalDatabasePath)
	legacyDataRoot := filepath.Dir(legacyDatabasePath)
	if canonicalDataRoot == legacyDataRoot {
		return nil
	}

	managedEntries := []string{
		filepath.Base(canonicalDatabasePath),
		filepath.Base(canonicalDatabasePath) + "-wal",
		filepath.Base(canonicalDatabasePath) + "-shm",
		"plugins",
		"render",
	}

	if err := os.MkdirAll(canonicalDataRoot, 0o755); err != nil {
		return fmt.Errorf("create canonical data directory %s: %w", canonicalDataRoot, err)
	}

	for _, entryName := range managedEntries {
		legacyEntryPath := filepath.Join(legacyDataRoot, entryName)
		info, statErr := os.Stat(legacyEntryPath)
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return fmt.Errorf("inspect legacy data entry %s: %w", legacyEntryPath, statErr)
		}

		canonicalEntryPath := filepath.Join(canonicalDataRoot, entryName)
		if _, err := os.Stat(canonicalEntryPath); err == nil {
			if logger != nil {
				logger.Warn(
					"legacy data entry left in place because canonical target already exists",
					"component", "app",
					"legacy_path", legacyEntryPath,
					"canonical_path", canonicalEntryPath,
				)
			}
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect canonical data entry %s: %w", canonicalEntryPath, err)
		}

		if err := os.Rename(legacyEntryPath, canonicalEntryPath); err != nil {
			return fmt.Errorf("migrate legacy data entry %s to %s: %w", legacyEntryPath, canonicalEntryPath, err)
		}

		if logger != nil {
			logger.Info(
				"migrated legacy data entry to canonical data root",
				"component", "app",
				"legacy_path", legacyEntryPath,
				"canonical_path", canonicalEntryPath,
				"is_dir", info.IsDir(),
			)
		}
	}

	removeEmptyDir(legacyDataRoot)
	return nil
}

func removeEmptyDir(path string) {
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) > 0 {
		return
	}
	_ = os.Remove(path)
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

	repoRoot, pluginSchemaPath, roots, err := pluginDiscoveryContext(options.SchemaPath)
	if err != nil {
		return pluginDiscoverySpec{}, err
	}
	return pluginDiscoverySpec{
		repoRoot:         repoRoot,
		pluginSchemaPath: pluginSchemaPath,
		roots:            roots,
	}, nil
}

func pluginDiscoveryContext(configSchemaPath string) (string, string, []plugins.ScanRoot, error) {
	absoluteConfigSchemaPath, err := filepath.Abs(configSchemaPath)
	if err != nil {
		return "", "", nil, fmt.Errorf("resolve config schema path %s: %w", configSchemaPath, err)
	}

	contractsDir := filepath.Dir(absoluteConfigSchemaPath)
	repoRoot := filepath.Dir(contractsDir)
	pluginSchemaPath := filepath.Join(contractsDir, "plugin-info.schema.json")

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
