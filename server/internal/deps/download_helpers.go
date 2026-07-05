package deps

import (
	"context"
	"reflect"
)

func downloadHTTPSFile(ctx context.Context, rawURL, destPath string) error {
	return downloadHTTPSFileWithProgress(ctx, rawURL, destPath, nil)
}

func downloadHTTPSFileWithProgress(ctx context.Context, rawURL, destPath string, progress func(downloadProgress)) error {
	return HTTPSFileWithProgress(ctx, rawURL, destPath, func(event DownloadProgress) {
		if progress == nil {
			return
		}
		progress(downloadProgress{
			DownloadedBytes: event.DownloadedBytes,
			TotalBytes:      event.TotalBytes,
			Progress:        event.Progress,
		})
	})
}

func downloadWithProgress(ctx context.Context, rawURL, destPath string, downloader func(context.Context, string, string) error, progress func(downloadProgress)) error {
	if downloader != nil {
		return downloader(ctx, rawURL, destPath)
	}
	return downloadHTTPSFileWithProgress(ctx, rawURL, destPath, progress)
}

func normalizedResourceSources(sources []ResourceSource) []ResourceSource {
	return NormalizeSources(sources)
}

func downloadSourceSummary(kind string, source ResourceSource) string {
	return SourceSummary(kind, source)
}

func sameFunction(left, right any) bool {
	if left == nil || right == nil {
		return false
	}
	leftValue := reflect.ValueOf(left)
	rightValue := reflect.ValueOf(right)
	if leftValue.Kind() != reflect.Func || rightValue.Kind() != reflect.Func {
		return false
	}
	return leftValue.Pointer() == rightValue.Pointer()
}
