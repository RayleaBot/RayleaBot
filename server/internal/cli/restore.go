package cli

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func runRestore(cmd Command) int {
	if len(cmd.Args) == 0 {
		cmd.Logger.Error("restore requires a backup file path: raylea-server restore <path>")
		return 1
	}
	backupPath := cmd.Args[0]

	reader, err := zip.OpenReader(backupPath)
	if err != nil {
		cmd.Logger.Error("open backup archive", "path", backupPath, "err", err.Error())
		return 1
	}
	defer reader.Close()

	// Validate manifest
	var manifest backupManifest
	manifestFound := false
	for _, f := range reader.File {
		if f.Name == "backup-manifest.json" {
			rc, err := f.Open()
			if err != nil {
				cmd.Logger.Error("read backup manifest", "err", err.Error())
				return 1
			}
			if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
				rc.Close()
				cmd.Logger.Error("parse backup manifest", "err", err.Error())
				return 1
			}
			rc.Close()
			manifestFound = true
			break
		}
	}

	if !manifestFound {
		cmd.Logger.Error("backup archive missing backup-manifest.json")
		return 1
	}
	if manifest.Version != "1" {
		cmd.Logger.Error("unsupported backup version", "version", manifest.Version)
		return 1
	}

	cmd.Logger.Info("restoring from backup",
		"path", backupPath,
		"created_at", manifest.CreatedAt,
		"items", len(manifest.Items),
	)

	configDir := filepath.Dir(cmd.ConfigPath)
	repoRoot := filepath.Dir(configDir)

	restored := 0
	for _, f := range reader.File {
		if f.Name == "backup-manifest.json" {
			continue
		}

		cleanName := filepath.Clean(f.Name)
		if filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, "..") {
			cmd.Logger.Warn("skip path traversal entry", "name", f.Name)
			continue
		}

		targetPath := filepath.Join(repoRoot, filepath.FromSlash(cleanName))

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				cmd.Logger.Warn("create directory", "path", targetPath, "err", err.Error())
			}
			continue
		}

		if err := restoreFile(f, targetPath); err != nil {
			cmd.Logger.Warn("restore file failed", "path", targetPath, "err", err.Error())
			continue
		}
		restored++
	}

	cmd.Logger.Info("restore completed",
		"restored_files", restored,
	)
	return 0
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
