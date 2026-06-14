package archive

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

type Progress struct {
	ExtractedEntries int
	TotalEntries     int
	Progress         int
}

func Extract(ctx context.Context, archivePath, archiveFormat, destRoot string) error {
	return ExtractWithProgress(ctx, archivePath, archiveFormat, destRoot, nil)
}

func ExtractWithProgress(ctx context.Context, archivePath, archiveFormat, destRoot string, progress func(Progress)) error {
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

func pathWithinRoot(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func progressPercent(done, total int64) int {
	if total <= 0 || done <= 0 {
		return 0
	}
	percent := int((done * 100) / total)
	if percent > 100 {
		return 100
	}
	if percent < 0 {
		return 0
	}
	return percent
}
