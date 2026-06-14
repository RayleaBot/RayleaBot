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
