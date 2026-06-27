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

	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
)

func runRestore(cmd Command) int {
	if len(cmd.Args) == 0 {
		cmd.Logger.Error("恢复备份失败：缺少备份文件路径，用法 raylea-server restore <path>")
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
	defer reader.Close()

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
				rc.Close()
				cmd.Logger.Error("解析备份清单失败："+backupPathDisplay, "path", backupPathDisplay, "err", displayLogError(repoRoot, err, backupPath))
				return 1
			}
			rc.Close()
			manifestFound = true
			break
		}
	}

	if !manifestFound {
		cmd.Logger.Error("备份压缩包缺少 backup-manifest.json："+backupPathDisplay, "path", backupPathDisplay)
		return 1
	}
	if manifest.Version != recovery.BackupManifestVersion {
		cmd.Logger.Error("备份版本不支持："+manifest.Version, "version", manifest.Version)
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

		targetPath, ok := restoreTargetPath(repoRoot, f.Name)
		if !ok {
			cmd.Logger.Warn("备份条目路径不安全，已跳过："+f.Name, "name", f.Name)
			continue
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				targetPathDisplay := displayLogPath(repoRoot, targetPath)
				cmd.Logger.Warn("创建恢复目录失败："+targetPathDisplay, "path", targetPathDisplay, "err", displayLogError(repoRoot, err, targetPath))
			}
			continue
		}

		if err := restoreFile(f, targetPath); err != nil {
			targetPathDisplay := displayLogPath(repoRoot, targetPath)
			cmd.Logger.Warn("恢复备份文件失败："+targetPathDisplay, "path", targetPathDisplay, "err", displayLogError(repoRoot, err, targetPath))
			continue
		}
		restored++
	}

	cmd.Logger.Info(fmt.Sprintf("备份恢复完成：恢复文件 %d 个，报告 %s", restored, summaryPathDisplay),
		"restored_files", restored,
		"recovery_summary", summaryPathDisplay,
	)
	return 0
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
		return fmt.Errorf("create parent dir: %w", err)
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}
