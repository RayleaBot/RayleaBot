package deps

import (
	"context"
	"fmt"
	"os"
	"strings"
)

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
