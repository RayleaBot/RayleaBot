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
		cmd.Logger.Error("create backups directory", "err", err.Error())
		return 1
	}

	backupPath := filepath.Join(backupDir, fmt.Sprintf("backup-%s.zip", timestamp))

	outFile, err := os.Create(backupPath)
	if err != nil {
		cmd.Logger.Error("create backup file", "path", backupPath, "err", err.Error())
		return 1
	}
	defer outFile.Close()

	w := zip.NewWriter(outFile)
	defer w.Close()

	var directories []recovery.BackupManifestDirectory

	// 1. config/user.yaml
	configFile := cmd.ConfigPath
	if err := addFileToZip(w, configFile, "config/user.yaml"); err != nil {
		cmd.Logger.Warn("skip config file", "path", configFile, "err", err.Error())
	} else {
		directories = append(directories, recovery.Directory("config/user.yaml", "config"))
		cmd.Logger.Info("backed up config", "path", configFile)
	}

	// 2. SQLite database
	var databasePath string
	dbPath, err := resolveDatabasePath(cmd.ConfigPath)
	if err == nil && fileExists(dbPath) {
		databasePath = dbPath
		archivePath := filepath.ToSlash(filepath.Join("data", filepath.Base(dbPath)))
		snapshotPath, err := storage.CreateSnapshot(context.Background(), dbPath)
		if err != nil {
			cmd.Logger.Error("create database snapshot", "path", dbPath, "err", err.Error())
			return 1
		}
		if err := addFileToZip(w, snapshotPath, archivePath); err != nil {
			cmd.Logger.Error("write database snapshot", "path", dbPath, "err", err.Error())
			return 1
		}
		directories = append(directories, recovery.Directory(archivePath, "database"))
		cmd.Logger.Info("backed up database", "path", dbPath)
	}

	// 3. plugins/installed/
	installedRoot := filepath.Join(repoRoot, "plugins", "installed")
	if info, err := os.Stat(installedRoot); err == nil && info.IsDir() {
		count, err := addDirToZip(w, installedRoot, "plugins/installed")
		if err != nil {
			cmd.Logger.Warn("skip plugins directory", "path", installedRoot, "err", err.Error())
		} else {
			directories = append(directories, recovery.Directory("plugins/installed", "plugins"))
			cmd.Logger.Info("backed up plugins", "files", count)
		}
	}

	// 4. Write manifest
	manifest := recovery.BuildBackupManifest(repoRoot, "offline")
	if len(directories) == 0 {
		directories = recovery.ScanRepoPaths(repoRoot, configFile, databasePath)
	}
	manifest.Directories = directories
	if err := addManifestToZip(w, manifest); err != nil {
		cmd.Logger.Error("write backup manifest", "err", err.Error())
		return 1
	}

	if err := w.Close(); err != nil {
		cmd.Logger.Error("finalize backup archive", "err", err.Error())
		return 1
	}
	outFile.Close()

	cmd.Logger.Info("backup completed", "path", backupPath, "directories", len(directories), "plugins", len(manifest.Plugins))
	return 0
}
