package deps

import (
	"context"

	depsarchive "github.com/RayleaBot/RayleaBot/server/internal/deps/archive"
)

func extractArchive(ctx context.Context, archivePath, archiveFormat, destRoot string) error {
	return depsarchive.Extract(ctx, archivePath, archiveFormat, destRoot)
}

func extractWithProgress(ctx context.Context, archivePath, archiveFormat, destRoot string, extractor func(context.Context, string, string, string) error, progress func(extractProgress)) error {
	if extractor != nil && !sameFunction(extractor, extractArchive) {
		return extractor(ctx, archivePath, archiveFormat, destRoot)
	}
	return depsarchive.ExtractWithProgress(ctx, archivePath, archiveFormat, destRoot, func(event depsarchive.Progress) {
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
