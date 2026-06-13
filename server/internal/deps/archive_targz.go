package deps

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func extractTarGz(archivePath, destRoot string) error {
	return extractTarGzWithProgress(archivePath, destRoot, nil)
}

func extractTarGzWithProgress(archivePath, destRoot string, progress func(extractProgress)) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	totalEntries, err := countTarGzEntries(archivePath)
	if err != nil {
		totalEntries = 0
	}
	reader := tar.NewReader(gzr)
	extractedEntries := 0
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			if progress != nil {
				progress(extractProgress{
					ExtractedEntries: extractedEntries,
					TotalEntries:     totalEntries,
					Progress:         100,
				})
			}
			return nil
		}
		if err != nil {
			return err
		}
		targetPath := filepath.Join(destRoot, filepath.FromSlash(header.Name))
		if !pathWithinRoot(destRoot, targetPath) {
			return fmt.Errorf("tar entry escapes destination: %s", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, 0:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, reader); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
		extractedEntries++
		if progress != nil {
			progress(extractProgress{
				ExtractedEntries: extractedEntries,
				TotalEntries:     totalEntries,
				Progress:         prepareProgressPercent(int64(extractedEntries), int64(totalEntries)),
			})
		}
	}
}

func countTarGzEntries(archivePath string) (int, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return 0, err
	}
	defer gzr.Close()
	reader := tar.NewReader(gzr)
	total := 0
	for {
		_, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return total, nil
		}
		if err != nil {
			return total, err
		}
		total++
	}
}
