package deps

import (
	"context"
	"reflect"
)

func downloadWithProgress(ctx context.Context, rawURL, destPath string, downloader func(context.Context, string, string) error, progress func(downloadProgress)) error {
	if downloader == nil || sameFunction(downloader, downloadHTTPSFile) {
		return downloadHTTPSFileWithProgress(ctx, rawURL, destPath, progress)
	}
	return downloader(ctx, rawURL, destPath)
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
