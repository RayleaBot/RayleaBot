package cli

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func runBackup(cmd Command) int {
	configDir := filepath.Dir(cmd.ConfigPath)
	repoRoot := filepath.Dir(configDir)

	timestamp := time.Now().UTC().Format("20060102-150405")
	backupDir := filepath.Join(repoRoot, "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		backupDirDisplay := displayLogPath(repoRoot, backupDir)
		cmd.Logger.Error("创建备份目录失败："+backupDirDisplay, "path", backupDirDisplay, "err", displayLogError(repoRoot, err, backupDir))
		return 1
	}

	backupPath := filepath.Join(backupDir, fmt.Sprintf("backup-%s.zip", timestamp))
	backupPathDisplay := displayLogPath(repoRoot, backupPath)

	outFile, err := os.Create(backupPath)
	if err != nil {
		cmd.Logger.Error("创建备份文件失败："+backupPathDisplay, "path", backupPathDisplay, "err", displayLogError(repoRoot, err, backupPath))
		return 1
	}
	defer outFile.Close()

	w := zip.NewWriter(outFile)
	defer w.Close()

	var directories []recovery.BackupManifestDirectory

	// 1. config/user.yaml
	configFile := cmd.ConfigPath
	configFileDisplay := displayLogPath(repoRoot, configFile)
	if err := addFileToZip(w, configFile, "config/user.yaml"); err != nil {
		cmd.Logger.Warn("备份配置文件失败，已跳过："+configFileDisplay, "path", configFileDisplay, "err", displayLogError(repoRoot, err, configFile))
	} else {
		directories = append(directories, recovery.Directory("config/user.yaml", "config"))
		cmd.Logger.Info("配置文件已备份："+configFileDisplay, "path", configFileDisplay)
	}

	// 2. SQLite database
	var databasePath string
	dbPath, err := resolveDatabasePath(cmd.ConfigPath)
	if err == nil && fileExists(dbPath) {
		databasePath = dbPath
		dbPathDisplay := displayLogPath(repoRoot, dbPath)
		archivePath := filepath.ToSlash(filepath.Join("data", filepath.Base(dbPath)))
		snapshotPath, err := storage.CreateSnapshot(context.Background(), dbPath)
		if err != nil {
			cmd.Logger.Error("创建数据库快照失败："+dbPathDisplay, "path", dbPathDisplay, "err", displayLogError(repoRoot, err, dbPath))
			return 1
		}
		snapshotPathDisplay := displayLogPath(repoRoot, snapshotPath)
		if err := addFileToZip(w, snapshotPath, archivePath); err != nil {
			cmd.Logger.Error("写入数据库快照到备份包失败："+dbPathDisplay, "path", dbPathDisplay, "snapshot_path", snapshotPathDisplay, "err", displayLogError(repoRoot, err, dbPath, snapshotPath))
			return 1
		}
		directories = append(directories, recovery.Directory(archivePath, "database"))
		cmd.Logger.Info("数据库已备份："+dbPathDisplay, "path", dbPathDisplay)
	}

	// 3. plugins/installed/
	installedRoot := filepath.Join(repoRoot, "plugins", "installed")
	if info, err := os.Stat(installedRoot); err == nil && info.IsDir() {
		installedRootDisplay := displayLogPath(repoRoot, installedRoot)
		count, err := addDirToZip(w, installedRoot, "plugins/installed")
		if err != nil {
			cmd.Logger.Warn("备份插件安装目录失败，已跳过："+installedRootDisplay, "path", installedRootDisplay, "err", displayLogError(repoRoot, err, installedRoot))
		} else {
			directories = append(directories, recovery.Directory("plugins/installed", "plugins"))
			cmd.Logger.Info(fmt.Sprintf("插件安装目录已备份：%s，文件数 %d", installedRootDisplay, count), "path", installedRootDisplay, "files", count)
		}
	}

	// 4. Write manifest
	manifest := recovery.BuildBackupManifest(repoRoot, "offline")
	if len(directories) == 0 {
		directories = recovery.ScanRepoPaths(repoRoot, configFile, databasePath)
	}
	manifest.Directories = directories
	if err := addManifestToZip(w, manifest); err != nil {
		cmd.Logger.Error("写入备份清单失败："+backupPathDisplay, "path", backupPathDisplay, "err", displayLogError(repoRoot, err, backupPath))
		return 1
	}

	if err := w.Close(); err != nil {
		cmd.Logger.Error("完成备份压缩包失败："+backupPathDisplay, "path", backupPathDisplay, "err", displayLogError(repoRoot, err, backupPath))
		return 1
	}
	outFile.Close()

	cmd.Logger.Info(fmt.Sprintf("备份完成：%s，目录 %d 个，插件 %d 个", backupPathDisplay, len(directories), len(manifest.Plugins)), "path", backupPathDisplay, "directories", len(directories), "plugins", len(manifest.Plugins))
	return 0
}
