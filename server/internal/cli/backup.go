package cli

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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

	// 3. data/** outside the canonical SQLite database and its transient sidecars.
	// The database itself is written from the verified snapshot above. All other
	// application and plugin state under data/ must survive an offline restore.
	dataRoot := filepath.Join(repoRoot, "data")
	if info, statErr := os.Stat(dataRoot); statErr == nil && info.IsDir() {
		dataRootDisplay := displayLogPath(repoRoot, dataRoot)
		skippedDatabasePaths := []string{databasePath, databasePath + "-wal", databasePath + "-shm"}
		count, addErr := addDirToZipFiltered(w, dataRoot, "data", func(path string, _ os.DirEntry) bool {
			for _, skipped := range skippedDatabasePaths {
				if skipped != "" && sameBackupPath(path, skipped) {
					return true
				}
			}
			return false
		})
		if addErr != nil {
			cmd.Logger.Warn("备份数据目录失败，已跳过："+dataRootDisplay, "path", dataRootDisplay, "err", displayLogError(repoRoot, addErr, dataRoot))
		} else {
			directories = append(directories, recovery.Directory("data", "data"))
			cmd.Logger.Info(fmt.Sprintf("数据目录已备份：%s，文件数 %d", dataRootDisplay, count), "path", dataRootDisplay, "files", count)
		}
	}

	// 4. plugins/installed/
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

	// 5. Write manifest
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

func sameBackupPath(left, right string) bool {
	leftAbsolute, leftErr := filepath.Abs(left)
	rightAbsolute, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return strings.EqualFold(filepath.Clean(leftAbsolute), filepath.Clean(rightAbsolute))
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func addManifestToZip(w *zip.Writer, manifest recovery.BackupManifest) error {
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	mw, err := w.Create("backup-manifest.json")
	if err != nil {
		return err
	}
	_, err = mw.Write(payload)
	return err
}

func addFileToZip(w *zip.Writer, srcPath, zipPath string) error {
	lstat, err := os.Lstat(srcPath)
	if err != nil {
		return err
	}
	if !lstat.Mode().IsRegular() {
		return fmt.Errorf("backup source is not a regular file: %s", srcPath)
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(zipPath)
	header.Method = zip.Deflate

	writer, err := w.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, f)
	return err
}

func addDirToZip(w *zip.Writer, srcRoot, zipPrefix string) (int, error) {
	return addDirToZipFiltered(w, srcRoot, zipPrefix, nil)
}

func addDirToZipFiltered(w *zip.Writer, srcRoot, zipPrefix string, skip func(string, os.DirEntry) bool) (int, error) {
	count := 0
	err := filepath.WalkDir(srcRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if skip != nil && skip(path, d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		zipPath := filepath.ToSlash(filepath.Join(zipPrefix, relPath))

		if d.IsDir() {
			_, err := w.Create(zipPath + "/")
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("backup source contains a non-regular file: %s", path)
		}

		if err := addFileToZip(w, path, zipPath); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}
