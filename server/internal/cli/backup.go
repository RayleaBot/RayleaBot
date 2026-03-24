package cli

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type backupManifest struct {
	Version   string `json:"version"`
	CreatedAt string `json:"created_at"`
	Items     []backupItem `json:"items"`
}

type backupItem struct {
	Label string `json:"label"`
	Path  string `json:"path"`
}

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

	var items []backupItem

	// 1. config/user.yaml
	configFile := cmd.ConfigPath
	if err := addFileToZip(w, configFile, "config/user.yaml"); err != nil {
		cmd.Logger.Warn("skip config file", "path", configFile, "err", err.Error())
	} else {
		items = append(items, backupItem{Label: "config", Path: "config/user.yaml"})
		cmd.Logger.Info("backed up config", "path", configFile)
	}

	// 2. SQLite database
	dbPath, err := resolveDatabasePath(cmd.ConfigPath)
	if err == nil {
		archivePath := filepath.ToSlash(filepath.Join("data", filepath.Base(dbPath)))
		if err := addFileToZip(w, dbPath, archivePath); err != nil {
			cmd.Logger.Warn("skip database file", "path", dbPath, "err", err.Error())
		} else {
			items = append(items, backupItem{Label: "database", Path: archivePath})
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
			items = append(items, backupItem{Label: "plugins", Path: "plugins/installed"})
			cmd.Logger.Info("backed up plugins", "files", count)
		}
	}

	// 4. Write manifest
	manifest := backupManifest{
		Version:   "1",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Items:     items,
	}
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
	mw, err := w.Create("backup-manifest.json")
	if err == nil {
		mw.Write(manifestBytes)
	}

	if err := w.Close(); err != nil {
		cmd.Logger.Error("finalize backup archive", "err", err.Error())
		return 1
	}
	outFile.Close()

	cmd.Logger.Info("backup completed", "path", backupPath, "items", len(items))
	return 0
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
