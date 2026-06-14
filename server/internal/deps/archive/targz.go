package archive

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func TarGz(archivePath, destRoot string) error {
	return TarGzWithProgress(archivePath, destRoot, nil)
}

func TarGzWithProgress(archivePath, destRoot string, progress func(Progress)) error {
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

	totalEntries, err := CountTarGzEntries(archivePath)
	if err != nil {
		totalEntries = 0
	}
	reader := tar.NewReader(gzr)
	extractedEntries := 0
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			if progress != nil {
				progress(Progress{
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
			progress(Progress{
				ExtractedEntries: extractedEntries,
				TotalEntries:     totalEntries,
				Progress:         progressPercent(int64(extractedEntries), int64(totalEntries)),
			})
		}
	}
}

func CountTarGzEntries(archivePath string) (int, error) {
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
