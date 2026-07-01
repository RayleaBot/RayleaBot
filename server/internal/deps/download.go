package deps

import (
	"context"

	depsdownload "github.com/RayleaBot/RayleaBot/server/internal/deps/download"
)

func downloadHTTPSFile(ctx context.Context, rawURL, destPath string) error {
	return downloadHTTPSFileWithProgress(ctx, rawURL, destPath, nil)
}

func downloadHTTPSFileWithProgress(ctx context.Context, rawURL, destPath string, progress func(downloadProgress)) error {
	return depsdownload.HTTPSFileWithProgress(ctx, rawURL, destPath, downloadProgressAdapter(progress))
}

func downloadWithProgress(ctx context.Context, rawURL, destPath string, downloader func(context.Context, string, string) error, progress func(downloadProgress)) error {
	if downloader == nil || sameFunction(downloader, downloadHTTPSFile) {
		return downloadHTTPSFileWithProgress(ctx, rawURL, destPath, progress)
	}
	return downloader(ctx, rawURL, destPath)
}

func downloadProgressAdapter(progress func(downloadProgress)) func(depsdownload.Progress) {
	return func(event depsdownload.Progress) {
		if progress == nil {
			return
		}
		progress(downloadProgress{
			DownloadedBytes: event.DownloadedBytes,
			TotalBytes:      event.TotalBytes,
			Progress:        event.Progress,
		})
	}
}

func sameFunction(left, right any) bool {
	return depsdownload.SameFunction(left, right)
}

func normalizedResourceSources(sources []ResourceSource) []ResourceSource {
	return depsdownload.NormalizeSources(sources)
}

func downloadSourceSummary(kind string, source ResourceSource) string {
	return depsdownload.SourceSummary(kind, source)
}
