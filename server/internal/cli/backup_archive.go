package cli

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
)

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
