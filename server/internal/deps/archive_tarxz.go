package deps

import (
	"context"

	depsarchive "github.com/RayleaBot/RayleaBot/server/internal/deps/archive"
)

func extractTarXzWithProgress(ctx context.Context, archivePath, destRoot string, progress func(extractProgress)) error {
	return depsarchive.TarXzWithProgress(ctx, archivePath, destRoot, func(event depsarchive.Progress) {
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
