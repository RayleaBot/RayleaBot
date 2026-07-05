package deps

import (
	"context"
	"fmt"
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

func extractArchive(ctx context.Context, archivePath, archiveFormat, destRoot string) error {
	return Extract(ctx, archivePath, archiveFormat, destRoot)
}

func extractWithProgress(ctx context.Context, archivePath, archiveFormat, destRoot string, extractor func(context.Context, string, string, string) error, progress func(extractProgress)) error {
	if extractor != nil && !sameFunction(extractor, extractArchive) {
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
