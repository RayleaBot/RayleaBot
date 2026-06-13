package deps

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
)

func downloadHTTPSFile(ctx context.Context, rawURL, destPath string) error {
	return downloadHTTPSFileWithProgress(ctx, rawURL, destPath, nil)
}

func downloadHTTPSFileWithProgress(ctx context.Context, rawURL, destPath string, progress func(downloadProgress)) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", response.StatusCode)
	}
	file, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, &progressReader{
		reader: response.Body,
		total:  response.ContentLength,
		notify: progress,
	})
	return err
}
