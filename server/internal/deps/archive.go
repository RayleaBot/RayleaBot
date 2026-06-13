package deps

import (
	"context"
	"fmt"
	"reflect"
)

func extractArchive(ctx context.Context, archivePath, archiveFormat, destRoot string) error {
	return extractWithProgress(ctx, archivePath, archiveFormat, destRoot, nil, nil)
}
func extractWithProgress(ctx context.Context, archivePath, archiveFormat, destRoot string, extractor func(context.Context, string, string, string) error, progress func(extractProgress)) error {
	if extractor != nil && reflect.ValueOf(extractor).Pointer() != reflect.ValueOf(extractArchive).Pointer() {
		return extractor(ctx, archivePath, archiveFormat, destRoot)
	}
	switch archiveFormat {
	case "zip":
		return extractZipWithProgress(archivePath, destRoot, progress)
	case "tar.gz":
		return extractTarGzWithProgress(archivePath, destRoot, progress)
	case "tar.xz":
		return extractTarXzWithProgress(ctx, archivePath, destRoot, progress)
	default:
		return fmt.Errorf("unsupported archive format %s", archiveFormat)
	}
}
