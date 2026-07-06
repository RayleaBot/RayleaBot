package deps

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

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

func HTTPSFile(ctx context.Context, rawURL, destPath string) error {
	return HTTPSFileWithProgress(ctx, rawURL, destPath, nil)
}

func HTTPSFileWithProgress(ctx context.Context, rawURL, destPath string, progress func(DownloadProgress)) error {
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

func ensureDownloadedArchiveWithProgress(
	ctx context.Context,
	archivePath,
	storeRoot string,
	resource *Resource,
	downloader func(context.Context, string, string) error,
	sourceSelector func(context.Context, []ResourceSource) []ResourceSource,
	reporter PrepareProgressReporter,
) (string, []string, error) {
	if err := verifyFileSHA256(archivePath, resource.SHA256); err == nil {
		emitPrepareProgress(reporter, PrepareProgress{
			Stage:    "download",
			Status:   "succeeded",
			Progress: 100,
			Summary:  managedResourceText(resource.Kind, "安装包已下载"),
		}.withResource(resource, archivePath, storeRoot))
		return "", nil, nil
	}
	tempPath := archivePath + ".download"
	var attempted []string
	var finalErr error
	downloadSources := normalizedResourceSources(resource.Sources)
	if len(downloadSources) > 1 && sourceSelector != nil {
		emitPrepareProgress(reporter, PrepareProgress{
			Stage:    "probe",
			Status:   "running",
			Summary:  "正在测试 " + managedResourceText(resource.Kind, "下载来源"),
			Progress: 0,
		}.withResource(resource, archivePath, storeRoot))
		selectedSources := sourceSelector(ctx, downloadSources)
		if len(selectedSources) > 0 {
			downloadSources = selectedSources
		}
		emitPrepareProgress(reporter, PrepareProgress{
			Stage:    "probe",
			Status:   "succeeded",
			Summary:  managedResourceText(resource.Kind, "下载来源已测试"),
			Progress: 100,
		}.withResource(resource, archivePath, storeRoot))
	}
	for _, source := range downloadSources {
		rawURL := strings.TrimSpace(source.URL)
		if rawURL == "" {
			continue
		}
		attempted = append(attempted, rawURL)
		emitPrepareProgress(reporter, PrepareProgress{
			Stage:       "download",
			Status:      "running",
			SourceLabel: strings.TrimSpace(source.Label),
			SourceURL:   rawURL,
			Summary:     downloadSourceSummary(resource.Kind, source),
		}.withResource(resource, archivePath, storeRoot))
		_ = os.Remove(tempPath)
		if err := downloadWithProgress(ctx, rawURL, tempPath, downloader, func(progress downloadProgress) {
			emitPrepareProgress(reporter, PrepareProgress{
				Stage:           "download",
				Status:          "running",
				SourceLabel:     strings.TrimSpace(source.Label),
				SourceURL:       rawURL,
				Progress:        progress.Progress,
				DownloadedBytes: progress.DownloadedBytes,
				TotalBytes:      progress.TotalBytes,
				Summary:         downloadSourceSummary(resource.Kind, source),
			}.withResource(resource, archivePath, storeRoot))
		}); err != nil {
			_ = os.Remove(tempPath)
			finalErr = fmt.Errorf("download deps resource %s from %s: %w", resource.Kind, rawURL, err)
			continue
		}
		emitPrepareProgress(reporter, PrepareProgress{
			Stage:       "verify",
			Status:      "running",
			SourceLabel: strings.TrimSpace(source.Label),
			SourceURL:   rawURL,
			Progress:    100,
			Summary:     "正在校验 " + managedResourceText(resource.Kind, "安装包"),
		}.withResource(resource, archivePath, storeRoot))
		if err := verifyFileSHA256(tempPath, resource.SHA256); err != nil {
			_ = os.Remove(tempPath)
			finalErr = fmt.Errorf("verify deps resource %s archive from %s: %w", resource.Kind, rawURL, err)
			continue
		}
		if err := os.Rename(tempPath, archivePath); err != nil {
			_ = os.Remove(tempPath)
			finalErr = fmt.Errorf("persist deps archive %s from %s: %w", resource.Kind, rawURL, err)
			continue
		}
		emitPrepareProgress(reporter, PrepareProgress{
			Stage:       "download",
			Status:      "succeeded",
			SourceLabel: strings.TrimSpace(source.Label),
			SourceURL:   rawURL,
			Progress:    100,
			Summary:     managedResourceText(resource.Kind, "安装包已下载"),
		}.withResource(resource, archivePath, storeRoot))
		return rawURL, attempted, nil
	}
	if finalErr == nil {
		finalErr = fmt.Errorf("download deps resource %s: no usable source configured", resource.Kind)
	}
	return "", attempted, finalErr
}

type progressReader struct {
	reader     io.Reader
	total      int64
	read       int64
	lastNotify int
	lastBytes  int64
	notify     func(DownloadProgress)
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.read += int64(n)
		r.emit(false)
	}
	if errors.Is(err, io.EOF) {
		r.emit(true)
	}
	return n, err
}

func (r *progressReader) emit(force bool) {
	if r.notify == nil {
		return
	}
	percent := progressPercent(r.read, r.total)
	if !force && r.total <= 0 && r.read-r.lastBytes < 1024*1024 {
		return
	}
	if !force && r.total > 0 && percent == r.lastNotify {
		return
	}
	r.lastNotify = percent
	r.lastBytes = r.read
	r.notify(DownloadProgress{
		DownloadedBytes: r.read,
		TotalBytes:      r.total,
		Progress:        percent,
	})
}

func progressPercent(done, total int64) int {
	if total <= 0 || done <= 0 {
		return 0
	}
	percent := int((done * 100) / total)
	if percent > 100 {
		return 100
	}
	if percent < 0 {
		return 0
	}
	return percent
}
