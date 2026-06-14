package system

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

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

func addOptionalFileToZip(writer *zip.Writer, sourcePath, archivePath string) error {
	if _, err := os.Stat(sourcePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return addFileToZip(writer, sourcePath, archivePath)
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
