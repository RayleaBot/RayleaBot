package cli

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
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
	if err == nil {
		databasePath = dbPath
		archivePath := filepath.ToSlash(filepath.Join("data", filepath.Base(dbPath)))
		if err := addFileToZip(w, dbPath, archivePath); err != nil {
			cmd.Logger.Warn("skip database file", "path", dbPath, "err", err.Error())
		} else {
			directories = append(directories, recovery.Directory(archivePath, "database"))
			cmd.Logger.Info("backed up database", "path", dbPath)
		}
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
	count := 0
	err := filepath.WalkDir(srcRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		zipPath := filepath.ToSlash(filepath.Join(zipPrefix, relPath))

		if d.IsDir() {
			// Skip hidden temp directories
			if len(d.Name()) > 1 && d.Name()[0] == '.' {
				return filepath.SkipDir
			}
			_, err := w.Create(zipPath + "/")
			return err
		}

		if err := addFileToZip(w, path, zipPath); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}
