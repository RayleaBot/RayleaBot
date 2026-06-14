package app

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func (s *systemService) SubmitSystemBackupTask() (string, error) {
	if s == nil || s.taskExecutor == nil {
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

func (s *systemService) createBackupArchive(ctx context.Context, progress tasks.ProgressReporter) (string, error) {
	repoRoot := s.state.repoRoot
	if repoRoot == "" {
		repoRoot = filepath.Dir(filepath.Dir(s.state.Summary.ConfigPath))
	}

	backupDir := filepath.Join(repoRoot, "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", &tasks.TaskError{Code: "plugin.internal_error", Message: "创建备份目录失败"}
	}

	timestamp := time.Now().UTC().Format("20060102-150405")
	archivePath := filepath.Join(backupDir, fmt.Sprintf("backup-%s.zip", timestamp))

	outFile, err := os.Create(archivePath)
	if err != nil {
		return "", &tasks.TaskError{Code: "plugin.internal_error", Message: "创建备份文件失败"}
	}
	defer outFile.Close()

	writer := zip.NewWriter(outFile)
	defer writer.Close()

	var directories []recovery.BackupManifestDirectory

	progress.Update(30, "写入配置与状态库")
	if err := addFileToZip(writer, s.state.Summary.ConfigPath, "config/user.yaml"); err == nil {
		directories = append(directories, recovery.Directory("config/user.yaml", "config"))
	}

	databasePath, err := resolveDatabasePath(s.state.Summary.ConfigPath, s.state.Config.Database.Path)
	if err == nil {
		databaseSnapshotPath, err := s.createDatabaseSnapshot(ctx, databasePath)
		if err != nil {
			return "", &tasks.TaskError{Code: "plugin.internal_error", Message: "创建数据库快照失败"}
		}
		archivePath := filepath.ToSlash(filepath.Join("data", filepath.Base(databasePath)))
		if err := addFileToZip(writer, databaseSnapshotPath, archivePath); err != nil {
			return "", &tasks.TaskError{Code: "plugin.internal_error", Message: "写入数据库快照失败"}
		}
		directories = append(directories, recovery.Directory(archivePath, "database"))

		spoolPath := logging.SpoolPathForDatabase(databasePath)
		spoolArchivePath := filepath.ToSlash(filepath.Join("data", filepath.Base(spoolPath)))
		if err := addOptionalFileToZip(writer, spoolPath, spoolArchivePath); err == nil {
			directories = append(directories, recovery.Directory(spoolArchivePath, "database"))
		}

		quarantinePath := filepath.Join(filepath.Dir(spoolPath), "management-logs.spool.quarantine.jsonl")
		quarantineArchivePath := filepath.ToSlash(filepath.Join("data", filepath.Base(quarantinePath)))
		if err := addOptionalFileToZip(writer, quarantinePath, quarantineArchivePath); err == nil {
			directories = append(directories, recovery.Directory(quarantineArchivePath, "database"))
		}
	}

	progress.Update(60, "写入插件目录")
	installedRoot := filepath.Join(repoRoot, "plugins", "installed")
	if info, err := os.Stat(installedRoot); err == nil && info.IsDir() {
		if _, err := addDirToZip(writer, installedRoot, "plugins/installed"); err == nil {
			directories = append(directories, recovery.Directory("plugins/installed", "plugins"))
		}
	}

	progress.Update(85, "写入备份清单")
	manifest := recovery.BuildBackupManifest(repoRoot, "online")
	manifest.Directories = directories
	if err := addJSONToZip(writer, "backup-manifest.json", manifest); err != nil {
		return "", &tasks.TaskError{Code: "plugin.internal_error", Message: "写入备份清单失败"}
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	progress.Update(95, "完成在线备份")
	if err := writer.Close(); err != nil {
		return "", &tasks.TaskError{Code: "plugin.internal_error", Message: "完成备份归档失败"}
	}
	if err := outFile.Close(); err != nil {
		return "", &tasks.TaskError{Code: "plugin.internal_error", Message: "关闭备份归档失败"}
	}

	return archivePath, nil
}

func (s *systemService) createDatabaseSnapshot(ctx context.Context, databasePath string) (string, error) {
	if _, err := os.Stat(databasePath); err != nil {
		return "", err
	}
	if s != nil && s.storage != nil && filepath.Clean(s.storage.Path) == filepath.Clean(databasePath) {
		return s.storage.CreateSnapshot(ctx)
	}
	return storage.CreateSnapshot(ctx, databasePath)
}
