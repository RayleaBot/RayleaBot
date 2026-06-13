package cli

import (
	"os"
	"path/filepath"
)

func runCleanup(cmd Command) int {
	configDir := filepath.Dir(cmd.ConfigPath)
	repoRoot := filepath.Dir(configDir)
	cleaned := 0

	installedRoot := filepath.Join(repoRoot, "plugins", "installed")
	entries, err := os.ReadDir(installedRoot)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			if len(name) > len(".plugin-install-") && name[:len(".plugin-install-")] == ".plugin-install-" {
				orphanPath := filepath.Join(installedRoot, name)
				if err := os.RemoveAll(orphanPath); err != nil {
					cmd.Logger.Warn("failed to remove orphaned install dir", "path", orphanPath, "err", err.Error())
				} else {
					cmd.Logger.Info("removed orphaned install directory", "path", orphanPath)
					cleaned++
				}
			}
		}
	}

	cacheRoot := filepath.Join(repoRoot, "cache", "downloads")
	if _, err := os.Stat(cacheRoot); err == nil {
		cacheEntries, err := os.ReadDir(cacheRoot)
		if err == nil {
			for _, entry := range cacheEntries {
				entryPath := filepath.Join(cacheRoot, entry.Name())
				if err := os.RemoveAll(entryPath); err != nil {
					cmd.Logger.Warn("failed to remove cache entry", "path", entryPath, "err", err.Error())
				} else {
					cleaned++
				}
			}
			if len(cacheEntries) > 0 {
				cmd.Logger.Info("cleared download cache", "entries", len(cacheEntries))
			}
		}
	}

	cmd.Logger.Info("cleanup completed", "cleaned_items", cleaned)
	return 0
}
