package deps

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ExtractProgress struct {
	ExtractedEntries int
	TotalEntries     int
	Progress         int
}

func Extract(ctx context.Context, archivePath, archiveFormat, destRoot string) error {
	return ExtractWithProgress(ctx, archivePath, archiveFormat, destRoot, nil)
}

func ExtractWithProgress(ctx context.Context, archivePath, archiveFormat, destRoot string, progress func(ExtractProgress)) error {
	switch archiveFormat {
	case "zip":
		return ZipWithProgress(archivePath, destRoot, progress)
	case "tar.gz":
		return TarGzWithProgress(archivePath, destRoot, progress)
	case "tar.xz":
		return TarXzWithProgress(ctx, archivePath, destRoot, progress)
	default:
		return fmt.Errorf("unsupported archive format %s", archiveFormat)
	}
}

func extractWithProgress(ctx context.Context, archivePath, archiveFormat, destRoot string, extractor func(context.Context, string, string, string) error, progress func(extractProgress)) error {
	if extractor != nil {
		return extractor(ctx, archivePath, archiveFormat, destRoot)
	}
	return ExtractWithProgress(ctx, archivePath, archiveFormat, destRoot, func(event ExtractProgress) {
		if progress == nil {
			return
		}
		progress(extractProgress{
			ExtractedEntries: event.ExtractedEntries,
			TotalEntries:     event.TotalEntries,
			Progress:         event.Progress,
		})
	})
}

func Zip(archivePath, destRoot string) error {
	return ZipWithProgress(archivePath, destRoot, nil)
}

func ZipWithProgress(archivePath, destRoot string, progress func(ExtractProgress)) error {
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
			progress(ExtractProgress{
				ExtractedEntries: index + 1,
				TotalEntries:     totalEntries,
				Progress:         progressPercent(int64(index+1), int64(totalEntries)),
			})
		}
	}
	if progress != nil {
		progress(ExtractProgress{
			ExtractedEntries: totalEntries,
			TotalEntries:     totalEntries,
			Progress:         100,
		})
	}
	return nil
}

func TarGz(archivePath, destRoot string) error {
	return TarGzWithProgress(archivePath, destRoot, nil)
}

func TarGzWithProgress(archivePath, destRoot string, progress func(ExtractProgress)) error {
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
				progress(ExtractProgress{
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
			progress(ExtractProgress{
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

func TarXzWithProgress(ctx context.Context, archivePath, destRoot string, progress func(ExtractProgress)) error {
	if progress != nil {
		progress(ExtractProgress{Progress: 0})
	}
	cmd := exec.CommandContext(ctx, "tar", "-xf", archivePath, "-C", destRoot)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) == 0 {
			return err
		}
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	if progress != nil {
		progress(ExtractProgress{Progress: 100})
	}
	return nil
}
