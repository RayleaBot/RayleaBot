package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/cli"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func (h *systemHTTPHandlers) handleSystemBackup() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.system == nil || h.system.taskExecutor == nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		taskID, err := h.system.taskExecutor.Submit("backup.create", "创建在线备份", func(ctx context.Context, progress tasks.ProgressReporter) (*tasks.ResultSummary, error) {
			progress.Update(10, "准备备份目录")
			archivePath, err := h.system.createBackupArchive(ctx, progress)
			if err != nil {
				return nil, err
			}
			return &tasks.ResultSummary{
				Summary: "在线备份已创建",
				Details: map[string]any{"archive_path": archivePath},
			}, nil
		})
		if err != nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		writeAuthJSON(w, http.StatusAccepted, taskAcceptedResponse{TaskID: taskID})
	}
}

func (h *systemHTTPHandlers) handleSystemDiagnosticsExport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		archive, err := h.system.buildDiagnosticsArchive(r.Context())
		if err != nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="rayleabot-diagnostics.zip"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(archive)
	}
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
		archivePath := filepath.ToSlash(filepath.Join("data", filepath.Base(databasePath)))
		if err := addFileToZip(writer, databasePath, archivePath); err == nil {
			directories = append(directories, recovery.Directory(archivePath, "database"))
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

func (s *systemService) buildDiagnosticsArchive(ctx context.Context) ([]byte, error) {
	buffer := &bytes.Buffer{}
	writer := zip.NewWriter(buffer)

	status := systemStatusResponse{
		Status:          s.systemStatus(),
		AdapterState:    string(stateOrIdle(s.adapter.Snapshot().State)),
		ActivePlugins:   s.activePluginCount(),
		UptimeSeconds:   s.uptimeSeconds(),
		RecoverySummary: s.state.recoverySummarySnapshot(),
	}
	if err := addJSONToZip(writer, "system-status.json", status); err != nil {
		return nil, err
	}
	if err := addJSONToZip(writer, "readiness.json", s.CurrentReadiness()); err != nil {
		return nil, err
	}
	doctorReport := cli.BuildDoctorReport(cli.Command{
		ConfigPath: s.state.Summary.ConfigPath,
		SchemaPath: s.state.Summary.SchemaPath,
	})
	if err := addJSONToZip(writer, "doctor.json", doctorReport); err != nil {
		return nil, err
	}
	if err := addJSONToZip(writer, "plugins.json", map[string]any{"items": s.plugins.List()}); err != nil {
		return nil, err
	}
	if err := addJSONToZip(writer, "config-summary.json", s.state.Summary); err != nil {
		return nil, err
	}
	if summary := s.state.recoverySummarySnapshot(); summary != nil {
		if err := addJSONToZip(writer, "recovery-summary.json", summary); err != nil {
			return nil, err
		}
	}
	if s.logRepository != nil {
		logs, err := s.logRepository.ListSummaries(ctx, logging.Query{Limit: 100})
		if err != nil {
			return nil, err
		}
		if err := addJSONToZip(writer, "recent-logs.json", map[string]any{"items": logs}); err != nil {
			return nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func addJSONToZip(writer *zip.Writer, path string, value any) error {
	bytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	entry, err := writer.Create(path)
	if err != nil {
		return err
	}
	_, err = entry.Write(bytes)
	return err
}

func addFileToZip(writer *zip.Writer, sourcePath, archivePath string) error {
	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(archivePath)
	header.Method = zip.Deflate

	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, file)
	return err
}

func addDirToZip(writer *zip.Writer, sourceRoot, archivePrefix string) (int, error) {
	count := 0
	err := filepath.WalkDir(sourceRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relativePath, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		archivePath := filepath.ToSlash(filepath.Join(archivePrefix, relativePath))

		if entry.IsDir() {
			if len(entry.Name()) > 1 && entry.Name()[0] == '.' {
				return filepath.SkipDir
			}
			if archivePath == archivePrefix {
				return nil
			}
			_, err := writer.Create(archivePath + "/")
			return err
		}

		if err := addFileToZip(writer, path, archivePath); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}
