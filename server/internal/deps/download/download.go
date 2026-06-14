package download

import (
	"context"
	"reflect"
)

type Progress struct {
	DownloadedBytes int64
	TotalBytes      int64
	Progress        int
}

func WithProgress(ctx context.Context, rawURL, destPath string, downloader func(context.Context, string, string) error, progress func(Progress)) error {
	if downloader == nil || SameFunction(downloader, HTTPSFile) {
		return HTTPSFileWithProgress(ctx, rawURL, destPath, progress)
	}
	return downloader(ctx, rawURL, destPath)
}

func SameFunction(left, right any) bool {
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
