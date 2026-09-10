package cli

import (
	"context"
	"errors"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"

	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
)

func runRestore(cmd Command) int {
	if len(cmd.Args) != 1 {
		cmd.Logger.Error("恢复备份失败：用法 raylea restore <backup-path>")
		return 1
	}
	repoRoot := runtimepaths.RootFromConfigPath(cmd.ConfigPath)
	result, err := recovery.Restore(context.Background(), recovery.RestoreOptions{
		ConfigPath: cmd.ConfigPath, ArchivePath: cmd.Args[0],
	})
	if err != nil {
		var failure *recovery.RestoreError
		recoveryDirectory := ""
		if errors.As(err, &failure) && failure.RecoveryDirectory != "" {
			recoveryDirectory = displayLogPath(repoRoot, failure.RecoveryDirectory)
		}
		cmd.Logger.Error("恢复备份未完成", "committed", result.Committed,
			"recovery_directory", recoveryDirectory,
			"err", displayLogError(repoRoot, err, cmd.Args[0], cmd.ConfigPath))
		return 1
	}
	cmd.Logger.Info("备份恢复完成", "restored_files", result.RestoredFiles,
		"database_path", displayLogPath(repoRoot, result.DatabasePath),
		"database_relocated", result.DatabaseRelocated,
		"recovery_summary", displayLogPath(repoRoot, recovery.SummaryPath(repoRoot)))
	return 0
}
