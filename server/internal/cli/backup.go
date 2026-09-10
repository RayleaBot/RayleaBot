package cli

import (
	"context"
	"path/filepath"
	"strings"

	backupsvc "github.com/RayleaBot/RayleaBot/server/internal/backup"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func runBackup(cmd Command) int {
	repoRoot := runtimepaths.RootFromConfigPath(cmd.ConfigPath)
	databasePath, err := runtimepaths.DatabaseFromConfig(cmd.ConfigPath)
	if err != nil {
		cmd.Logger.Error("解析数据库路径失败", "err", displayLogError(repoRoot, err, cmd.ConfigPath))
		return 1
	}
	result, err := backupsvc.Create(context.Background(), backupsvc.Options{
		RepoRoot:       repoRoot,
		ConfigPath:     cmd.ConfigPath,
		DatabasePath:   databasePath,
		Consistency:    "offline",
		CreateSnapshot: storage.CreateSnapshot,
		Now:            cmd.Now,
	})
	if err != nil {
		cmd.Logger.Error("创建离线备份失败", "err", displayLogError(repoRoot, err, cmd.ConfigPath, databasePath))
		return 1
	}
	backupPathDisplay := displayLogPath(repoRoot, result.ArchivePath)
	cmd.Logger.Info(
		"备份完成",
		"path", backupPathDisplay,
		"directories", len(result.Manifest.Directories),
		"plugins", len(result.Manifest.Plugins),
	)
	return 0
}

func sameBackupPath(left, right string) bool {
	leftAbsolute, leftErr := filepath.Abs(left)
	rightAbsolute, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return strings.EqualFold(filepath.Clean(leftAbsolute), filepath.Clean(rightAbsolute))
}
