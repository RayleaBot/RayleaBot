package deps

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func extractZip(archivePath, destRoot string) error {
	return extractZipWithProgress(archivePath, destRoot, nil)
}

func extractZipWithProgress(archivePath, destRoot string, progress func(extractProgress)) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	totalEntries := len(reader.File)
	for index, file := range reader.File {
		targetPath := filepath.Join(destRoot, filepath.FromSlash(file.Name))
		if !pathWithinRoot(destRoot, targetPath) {
			return fmt.Errorf("zip entry escapes destination: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			in.Close()
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			in.Close()
			return err
		}
		out.Close()
		in.Close()
		if progress != nil {
			progress(extractProgress{
				ExtractedEntries: index + 1,
				TotalEntries:     totalEntries,
				Progress:         prepareProgressPercent(int64(index+1), int64(totalEntries)),
			})
		}
	}
	if progress != nil {
		progress(extractProgress{
			ExtractedEntries: totalEntries,
			TotalEntries:     totalEntries,
			Progress:         100,
		})
	}
	return nil
}
