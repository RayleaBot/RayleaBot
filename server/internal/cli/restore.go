package cli

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"gopkg.in/yaml.v3"
)

func runRestore(cmd Command) int {
	if len(cmd.Args) != 1 {
		cmd.Logger.Error("恢复备份失败：用法 raylea restore <backup-path>")
		return 1
	}
	configDir := filepath.Dir(cmd.ConfigPath)
	repoRoot := filepath.Dir(configDir)
	backupPath := cmd.Args[0]
	backupPathDisplay := displayLogPath(repoRoot, backupPath)

	reader, err := zip.OpenReader(backupPath)
	if err != nil {
		cmd.Logger.Error("打开备份压缩包失败："+backupPathDisplay, "path", backupPathDisplay, "err", displayLogError(repoRoot, err, backupPath))
		return 1
	}
	defer func(release func() error) { _ = release() }(reader.Close)

	// Validate manifest
	var manifest recovery.BackupManifest
	manifestFound := false
	for _, f := range reader.File {
		if f.Name == "backup-manifest.json" {
			rc, err := f.Open()
			if err != nil {
				cmd.Logger.Error("读取备份清单失败："+backupPathDisplay, "path", backupPathDisplay, "err", displayLogError(repoRoot, err, backupPath))
				return 1
			}
			if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
				_ = rc.Close()
				cmd.Logger.Error("解析备份清单失败："+backupPathDisplay, "path", backupPathDisplay, "err", displayLogError(repoRoot, err, backupPath))
				return 1
			}
			_ = rc.Close()
			manifestFound = true
			break
		}
	}

	if !manifestFound {
		cmd.Logger.Error("备份压缩包缺少 backup-manifest.json："+backupPathDisplay, "path", backupPathDisplay)
		return 1
	}
	if err := recovery.ValidateBackupManifest(manifest); err != nil {
		cmd.Logger.Error("备份清单不符合正式契约："+backupPathDisplay, "path", backupPathDisplay, "err", err.Error())
		return 1
	}
	summary := recovery.EvaluateRestore(manifest, repoRoot)
	summaryPath := recovery.SummaryPath(repoRoot)
	summaryPathDisplay := displayLogPath(repoRoot, summaryPath)
	if err := recovery.SaveSummary(repoRoot, summary); err != nil {
		cmd.Logger.Error("写入恢复兼容性报告失败："+summaryPathDisplay, "path", summaryPathDisplay, "err", displayLogError(repoRoot, err, summaryPath))
		return 1
	}
	if summary.Status == "blocked" {
		cmd.Logger.Error("恢复备份被兼容性检查阻止："+backupPathDisplay, "path", backupPathDisplay, "issues", len(summary.Issues))
		return 1
	}
	if manifest.Version != recovery.BackupManifestVersion {
		cmd.Logger.Error("备份版本不支持："+manifest.Version, "version", manifest.Version)
		return 1
	}
	databaseEntries := make(map[string]struct{})
	for _, directory := range manifest.Directories {
		if directory.Label == "database" {
			databaseEntries[path.Clean(strings.ReplaceAll(directory.Path, "\\", "/"))] = struct{}{}
		}
	}
	databasePath := ""
	var restoredConfig map[string]any
	if len(databaseEntries) > 0 {
		restoredConfig, databasePath, err = archivedRestoreConfiguration(reader.File, cmd.ConfigPath, cmd.SchemaPath)
		if err != nil {
			cmd.Logger.Error("解析恢复目标数据库路径失败", "err", displayLogError(repoRoot, err, cmd.ConfigPath))
			return 1
		}
	}

	cmd.Logger.Info("开始从备份恢复："+backupPathDisplay,
		"path", backupPathDisplay,
		"created_at", manifest.CreatedAt,
		"directories", len(manifest.Directories),
		"operation", summary.Operation,
	)

	restored := 0
	for _, f := range reader.File {
		if f.Name == "backup-manifest.json" {
			continue
		}

		targetPath, ok := restoreManifestTargetPath(repoRoot, databasePath, databaseEntries, f.Name)
		if !ok {
			cmd.Logger.Warn("备份条目路径不安全，已跳过："+f.Name, "name", f.Name)
			continue
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				targetPathDisplay := displayLogPath(repoRoot, targetPath)
				cmd.Logger.Error("创建恢复目录失败："+targetPathDisplay, "path", targetPathDisplay, "err", displayLogError(repoRoot, err, targetPath))
				return 1
			}
			continue
		}

		var writeErr error
		if f.Name == "config/user.yaml" && restoredConfig != nil {
			_, _, writeErr = config.SaveDocument(cmd.ConfigPath, cmd.SchemaPath, restoredConfig)
		} else {
			writeErr = restoreFile(f, targetPath)
		}
		if err := writeErr; err != nil {
			targetPathDisplay := displayLogPath(repoRoot, targetPath)
			cmd.Logger.Error("恢复备份文件失败："+targetPathDisplay, "path", targetPathDisplay, "err", displayLogError(repoRoot, err, targetPath))
			return 1
		}
		restored++
	}

	cmd.Logger.Info(fmt.Sprintf("备份恢复完成：恢复文件 %d 个，报告 %s", restored, summaryPathDisplay),
		"restored_files", restored,
		"recovery_summary", summaryPathDisplay,
	)
	return 0
}

func restoreManifestTargetPath(repoRoot, databasePath string, databaseEntries map[string]struct{}, entryName string) (string, bool) {
	normalized := path.Clean(strings.ReplaceAll(strings.TrimSpace(entryName), "\\", "/"))
	if _, isDatabase := databaseEntries[normalized]; isDatabase {
		if strings.TrimSpace(databasePath) == "" || strings.HasSuffix(strings.TrimSpace(entryName), "/") {
			return "", false
		}
		return filepath.Clean(databasePath), true
	}
	return restoreTargetPath(repoRoot, entryName)
}

func restoreTargetPath(repoRoot string, entryName string) (string, bool) {
	normalized := strings.ReplaceAll(strings.TrimSpace(entryName), "\\", "/")
	if normalized == "" || strings.HasPrefix(normalized, "/") {
		return "", false
	}
	cleanName := strings.TrimSuffix(path.Clean(normalized), "/")
	if !slashPathIsLocal(cleanName) {
		return "", false
	}
	localName, err := filepath.Localize(cleanName)
	if err != nil || !filepath.IsLocal(localName) {
		return "", false
	}
	targetPath := filepath.Join(repoRoot, localName)
	return targetPath, pathWithinRoot(repoRoot, targetPath)
}

func slashPathIsLocal(value string) bool {
	if value == "" || value == "." || strings.HasPrefix(value, "/") {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func pathWithinRoot(root, candidate string) bool {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absoluteCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(absoluteRoot, absoluteCandidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func restoreFile(f *zip.File, targetPath string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	reader, err := f.Open()
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()
	file, err := os.CreateTemp(filepath.Dir(targetPath), ".restore-*")
	if err != nil {
		return err
	}
	temporaryPath := file.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if _, err := io.Copy(file, reader); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Chmod(f.Mode().Perm()); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, targetPath)
}

// The archived configuration supplies the database destination even when the
// restore target has never been initialized.
func archivedRestoreConfiguration(entries []*zip.File, configPath, schemaPath string) (map[string]any, string, error) {
	for _, entry := range entries {
		if entry.Name != "config/user.yaml" {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			return nil, "", err
		}
		payload, readErr := io.ReadAll(io.LimitReader(reader, 1024*1024+1))
		closeErr := reader.Close()
		if readErr != nil {
			return nil, "", readErr
		}
		if closeErr != nil {
			return nil, "", closeErr
		}
		if len(payload) > 1024*1024 {
			return nil, "", fmt.Errorf("archived config exceeds size limit")
		}
		var document map[string]any
		if err := yaml.Unmarshal(payload, &document); err != nil {
			return nil, "", err
		}
		typed, _, document, err := config.NormalizeDocument(configPath, schemaPath, document)
		if err != nil {
			return nil, "", err
		}
		name := strings.ReplaceAll(strings.TrimSpace(typed.Database.Path), "\\", "/")
		if strings.HasPrefix(name, "/") || (len(name) > 1 && name[1] == ':') {
			name = "data/rayleabot.db"
		}
		if !slashPathIsLocal(name) || strings.Contains(name, ":") {
			return nil, "", fmt.Errorf("archived database path is not a portable relative path")
		}
		first, _, nested := strings.Cut(name, "/")
		if !nested || strings.HasPrefix(first, ".") {
			return nil, "", fmt.Errorf("archived database must be inside a runtime data directory")
		}
		switch strings.ToLower(first) {
		case "config", "plugins", "logs", "cache", "backups", "templates", "web", ".deps":
			return nil, "", fmt.Errorf("archived database path targets a protected directory")
		}
		document["database"].(map[string]any)["path"] = name
		databasePath, valid := restoreTargetPath(filepath.Dir(filepath.Dir(configPath)), name)
		if !valid {
			return nil, "", fmt.Errorf("archived database path is not local")
		}
		return document, databasePath, nil
	}
	return nil, "", fmt.Errorf("backup database requires an archived configuration")
}
