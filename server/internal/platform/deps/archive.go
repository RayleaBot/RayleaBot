package deps

import "context"

type ExtractProgress struct {
	ExtractedEntries int
	TotalEntries     int
	Progress         int
}

func extractWithProgress(ctx context.Context, archivePath, archiveFormat, destRoot string, extractor func(context.Context, string, string, string) error, progress func(extractProgress)) error {
	if extractor != nil {
		return extractor(ctx, archivePath, archiveFormat, destRoot)
	}
	return ExtractWithProgress(ctx, archivePath, archiveFormat, destRoot, func(event ExtractProgress) {
		if progress != nil {
			progress(extractProgress(event))
		}
	})
}
