package deps

import (
	"context"

	depsdownload "github.com/RayleaBot/RayleaBot/server/internal/deps/download"
)

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
