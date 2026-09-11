package deps

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/fsguard"
)

const maxRuntimeArchiveBytes int64 = 2 << 30

var runtimeHTTPClient = &http.Client{
	Timeout: 30 * time.Minute,
	CheckRedirect: func(request *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("too many resource redirects")
		}
		return validateResourceURL(request.URL)
	},
}

func validateResourceURL(value *url.URL) error {
	if value.Scheme != "https" || value.Hostname() == "" || value.User != nil || value.Fragment != "" {
		return errors.New("resource download requires an HTTPS URL without credentials or fragment")
	}
	return nil
}

type DownloadProgress struct {
	DownloadedBytes int64
	TotalBytes      int64
	Progress        int
}

type archiveVerificationError struct{ cause error }

func (e *archiveVerificationError) Error() string { return e.cause.Error() }
func (e *archiveVerificationError) Unwrap() error { return e.cause }

func HTTPSFileWithProgress(ctx context.Context, rawURL, destPath string, progress func(DownloadProgress)) (err error) {
	return downloadRuntimeHTTP(ctx, runtimeHTTPClient, rawURL, destPath, maxRuntimeArchiveBytes, progress)
}

func downloadRuntimeHTTP(ctx context.Context, client *http.Client, rawURL, destPath string, limit int64, progress func(DownloadProgress)) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	idleTimer := time.AfterFunc(30*time.Second, cancel)
	defer idleTimer.Stop()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	if err := validateResourceURL(request.URL); err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer func(release func() error) { _ = release() }(response.Body.Close)
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", response.StatusCode)
	}
	if response.ContentLength > limit {
		return fsguard.ErrSizeLimit
	}
	file, err := os.OpenFile(destPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, file.Close())
		if err != nil {
			err = errors.Join(err, os.Remove(destPath))
		}
	}()
	written, err := fsguard.CopyAtMost(ctx, file, &progressReader{
		reader: response.Body,
		total:  response.ContentLength,
		notify: func(event DownloadProgress) {
			if progress != nil {
				progress(event)
			}
		},
		onRead: func() { idleTimer.Reset(30 * time.Second) },
	}, limit)
	if err == nil && response.ContentLength >= 0 && written != response.ContentLength {
		err = fmt.Errorf("resource size mismatch: expected %d, received %d", response.ContentLength, written)
	}
	if err == nil {
		err = file.Sync()
	}
	return err
}

func downloadWithProgress(ctx context.Context, rawURL, destPath string, downloader func(context.Context, string, string) error, progress func(DownloadProgress)) error {
	if downloader != nil {
		return downloader(ctx, rawURL, destPath)
	}
	return HTTPSFileWithProgress(ctx, rawURL, destPath, progress)
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
	if err := VerifyFileSHA256(archivePath, resource.SHA256); err == nil {
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
	downloadSources := NormalizeSources(resource.Sources)
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
		if err := ctx.Err(); err != nil {
			return "", attempted, err
		}
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
			Summary:     SourceSummary(resource.Kind, source),
		}.withResource(resource, archivePath, storeRoot))
		_ = os.Remove(tempPath)
		if err := downloadWithProgress(ctx, rawURL, tempPath, downloader, func(progress DownloadProgress) {
			emitPrepareProgress(reporter, PrepareProgress{
				Stage:           "download",
				Status:          "running",
				SourceLabel:     strings.TrimSpace(source.Label),
				SourceURL:       rawURL,
				Progress:        progress.Progress,
				DownloadedBytes: progress.DownloadedBytes,
				TotalBytes:      progress.TotalBytes,
				Summary:         SourceSummary(resource.Kind, source),
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
		if err := VerifyFileSHA256(tempPath, resource.SHA256); err != nil {
			_ = os.Remove(tempPath)
			finalErr = &archiveVerificationError{cause: fmt.Errorf("verify deps resource %s archive from %s: %w", resource.Kind, rawURL, err)}
			continue
		}
		if err := os.Rename(tempPath, archivePath); err != nil {
			_ = os.Remove(tempPath)
			finalErr = &archiveVerificationError{cause: fmt.Errorf("persist deps archive %s from %s: %w", resource.Kind, rawURL, err)}
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
	onRead     func()
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		if r.onRead != nil {
			r.onRead()
		}
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
