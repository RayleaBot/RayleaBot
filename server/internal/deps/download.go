package deps

import "context"

type DownloadProgress struct {
	DownloadedBytes int64
	TotalBytes      int64
	Progress        int
}

func WithProgress(ctx context.Context, rawURL, destPath string, downloader func(context.Context, string, string) error, progress func(DownloadProgress)) error {
	if downloader != nil {
		return downloader(ctx, rawURL, destPath)
	}
	return HTTPSFileWithProgress(ctx, rawURL, destPath, progress)
}
