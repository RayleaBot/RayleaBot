package deps

import "context"

func ensureDownloadedArchive(ctx context.Context, archivePath string, resource *Resource, downloader func(context.Context, string, string) error) (string, []string, error) {
	sourceSelector := selectDownloadSources
	if downloader != nil && !sameFunction(downloader, downloadHTTPSFile) {
		sourceSelector = nil
	}
	return ensureDownloadedArchiveWithProgress(ctx, archivePath, StoreRoot("", resource), resource, downloader, sourceSelector, nil)
}
