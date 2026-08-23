package system

import (
	"context"
	"os"
	"path/filepath"

	backupsvc "github.com/RayleaBot/RayleaBot/server/internal/backup"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func (s *Service) SubmitSystemBackupTask() (string, error) {
	if s.taskExecutor == nil {
		return "", errSystemTaskUnavailable
	}
	return s.taskExecutor.Submit("backup.create", "创建在线备份", func(ctx context.Context, progress tasks.ProgressReporter) (*tasks.ResultSummary, error) {
		progress.Update(10, "准备备份目录")
		archivePath, err := s.createBackupArchive(ctx, progress)
		if err != nil {
			return nil, err
		}
		return &tasks.ResultSummary{
			Summary: "在线备份已创建",
			Details: map[string]any{"archive_path": archivePath},
		}, nil
	})
}

func (s *Service) createBackupArchive(ctx context.Context, progress tasks.ProgressReporter) (string, error) {
	repoRoot := s.repoRootPath()
	if repoRoot == "" {
		repoRoot = filepath.Dir(filepath.Dir(s.summary().ConfigPath))
	}

	databasePath, err := s.databasePath(s.summary().ConfigPath, s.config().Database.Path)
	if err != nil {
		return "", &tasks.TaskError{Code: "plugin.internal_error", Message: "解析数据库路径失败"}
	}
	result, err := backupsvc.Create(ctx, backupsvc.Options{
		RepoRoot:       repoRoot,
		ConfigPath:     s.summary().ConfigPath,
		DatabasePath:   databasePath,
		Consistency:    "online",
		CreateSnapshot: s.createDatabaseSnapshot,
		Progress:       progress.Update,
	})
	if err != nil {
		return "", &tasks.TaskError{Code: "plugin.internal_error", Message: "创建在线备份失败"}
	}
	return result.ArchivePath, nil
}

func (s *Service) createDatabaseSnapshot(ctx context.Context, databasePath string) (string, error) {
	if _, err := os.Stat(databasePath); err != nil {
		return "", err
	}
	if s != nil && s.storage != nil && filepath.Clean(s.storage.Path) == filepath.Clean(databasePath) {
		return s.storage.CreateSnapshot(ctx)
	}
	return storage.CreateSnapshot(ctx, databasePath)
}
