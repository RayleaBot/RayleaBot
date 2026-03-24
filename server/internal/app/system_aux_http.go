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

	"rayleabot/server/internal/logging"
	"rayleabot/server/internal/tasks"
)

type backupManifest struct {
	Version   string       `json:"version"`
	CreatedAt string       `json:"created_at"`
	Items     []backupItem `json:"items"`
}

type backupItem struct {
	Label string `json:"label"`
	Path  string `json:"path"`
}

func (a *App) handleSystemBackup() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a == nil || a.taskExecutor == nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		taskID, err := a.taskExecutor.Submit("backup.create", "创建在线备份", func(ctx context.Context, progress tasks.ProgressReporter) (*tasks.ResultSummary, error) {
			progress.Update(10, "准备备份目录")
			archivePath, err := a.createBackupArchive(ctx, progress)
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

func (a *App) handleSystemDiagnosticsExport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		archive, err := a.buildDiagnosticsArchive(r.Context())
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

func (a *App) createBackupArchive(ctx context.Context, progress tasks.ProgressReporter) (string, error) {
	repoRoot := a.repoRoot
	if repoRoot == "" {
		repoRoot = filepath.Dir(filepath.Dir(a.Summary.ConfigPath))
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

	var items []backupItem

	progress.Update(30, "写入配置与状态库")
	if err := addFileToZip(writer, a.Summary.ConfigPath, "config/user.yaml"); err == nil {
		items = append(items, backupItem{Label: "config", Path: "config/user.yaml"})
	}

	databasePath, err := resolveDatabasePath(a.Summary.ConfigPath, a.Config.Database.Path)
	if err == nil {
		archivePath := filepath.ToSlash(filepath.Join("data", filepath.Base(databasePath)))
		if err := addFileToZip(writer, databasePath, archivePath); err == nil {
			items = append(items, backupItem{Label: "database", Path: archivePath})
		}
	}

	progress.Update(60, "写入插件目录")
	installedRoot := filepath.Join(repoRoot, "plugins", "installed")
	if info, err := os.Stat(installedRoot); err == nil && info.IsDir() {
		if _, err := addDirToZip(writer, installedRoot, "plugins/installed"); err == nil {
			items = append(items, backupItem{Label: "plugins", Path: "plugins/installed"})
		}
	}

	progress.Update(85, "写入备份清单")
	manifest := backupManifest{
		Version:   "1",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Items:     items,
	}
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

func (a *App) buildDiagnosticsArchive(ctx context.Context) ([]byte, error) {
	buffer := &bytes.Buffer{}
	writer := zip.NewWriter(buffer)

	status := systemStatusResponse{
		Status:        a.systemStatus(),
		AdapterState:  string(stateOrIdle(a.Adapter.Snapshot().State)),
		ActivePlugins: a.activePluginCount(),
		UptimeSeconds: a.uptimeSeconds(),
	}
	if err := addJSONToZip(writer, "system-status.json", status); err != nil {
		return nil, err
	}
	if err := addJSONToZip(writer, "readiness.json", a.currentReadiness()); err != nil {
		return nil, err
	}
	if err := addJSONToZip(writer, "plugins.json", map[string]any{"items": a.Plugins.List()}); err != nil {
		return nil, err
	}
	if err := addJSONToZip(writer, "config-summary.json", a.Summary); err != nil {
		return nil, err
	}
	if a.LogRepository != nil {
		logs, err := a.LogRepository.ListSummaries(ctx, logging.Query{Limit: 100})
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
