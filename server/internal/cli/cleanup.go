package cli

import (
	"fmt"
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
				orphanPathDisplay := displayLogPath(repoRoot, orphanPath)
				if err := os.RemoveAll(orphanPath); err != nil {
					cmd.Logger.Warn("清理遗留插件安装目录失败："+orphanPathDisplay, "path", orphanPathDisplay, "err", displayLogError(repoRoot, err, orphanPath))
				} else {
					cmd.Logger.Info("遗留插件安装目录已清理："+orphanPathDisplay, "path", orphanPathDisplay)
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
				entryPathDisplay := displayLogPath(repoRoot, entryPath)
				if err := os.RemoveAll(entryPath); err != nil {
					cmd.Logger.Warn("清理下载缓存条目失败："+entryPathDisplay, "path", entryPathDisplay, "err", displayLogError(repoRoot, err, entryPath))
				} else {
					cleaned++
				}
			}
			if len(cacheEntries) > 0 {
				cacheRootDisplay := displayLogPath(repoRoot, cacheRoot)
				cmd.Logger.Info(fmt.Sprintf("下载缓存已清理：%s，条目 %d 个", cacheRootDisplay, len(cacheEntries)), "path", cacheRootDisplay, "entries", len(cacheEntries))
			}
		}
	}

	cmd.Logger.Info(fmt.Sprintf("清理完成，共处理 %d 项", cleaned), "cleaned_items", cleaned)
	return 0
}
