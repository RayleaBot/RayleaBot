package deps

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (m *Manager) Prepare(ctx context.Context, kind string) (*PreparedResource, error) {
	report, err := m.PrepareWithReport(ctx, kind)
	if err != nil {
		return nil, err
	}
	return &PreparedResource{
		Resource:    report.Resource,
		Root:        report.StoreRoot,
		Entrypoints: report.Entrypoints,
	}, nil
}

func (m *Manager) PrepareWithReport(ctx context.Context, kind string) (*PrepareReport, error) {
	return m.PrepareWithReportOptions(ctx, kind, PrepareOptions{})
}

func (m *Manager) PrepareWithReportOptions(ctx context.Context, kind string, options PrepareOptions) (*PrepareReport, error) {
	if m == nil {
		return nil, errors.New("deps manager is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	manifest, resource, err := m.currentResource(kind)
	if err != nil {
		if report, ok := m.prepareSystemChromiumIfAvailable(ctx, kind, nil, nil, options.Progress); ok {
			return report, nil
		}
		return nil, m.classifyBootstrapErrorWithProgress(options.Progress, kind, nil, "manifest", "", nil, err)
	}
	report := &PrepareReport{
		Kind:        kind,
		Resource:    *resource,
		ArchivePath: filepath.Join(CacheRoot(m.repoRoot), resource.ID+"-"+resource.Version+archiveSuffix(resource.ArchiveFormat)),
		StoreRoot:   StoreRoot(m.repoRoot, resource),
	}
	emitPrepareProgress(options.Progress, PrepareProgress{
		Stage:   "inspect",
		Status:  "running",
		Summary: "正在检查 " + managedResourceLabel(kind),
	}.withResource(resource, report.ArchivePath, report.StoreRoot))
	if !manifest.HasPlatform(CurrentPlatform()) {
		if report, ok := m.prepareSystemChromiumIfAvailable(ctx, kind, report, resource, options.Progress); ok {
			return report, nil
		}
		return nil, m.classifyBootstrapErrorWithProgress(options.Progress, kind, resource, "manifest", "", nil, fmt.Errorf("deps manifest does not include current platform %s", CurrentPlatform()))
	}
	if !ResourceMetadataComplete(resource) {
		if report, ok := m.prepareSystemChromiumIfAvailable(ctx, kind, report, resource, options.Progress); ok {
			return report, nil
		}
		return nil, m.classifyBootstrapErrorWithProgress(options.Progress, kind, resource, "manifest", "", nil, fmt.Errorf("deps resource %s for %s is not bootstrap-ready", kind, CurrentPlatform()))
	}

	if prepared, err := m.resolvePreparedManifestResource(manifest, resource); err == nil {
		report.UsedPreparedStore = true
		report.Entrypoints = prepared.Entrypoints
		report.PreparedEntrypoint = primaryEntrypoint(prepared)
		emitPrepareProgress(options.Progress, PrepareProgress{
			Stage:    "complete",
			Status:   "succeeded",
			Progress: 100,
			Summary:  managedResourceText(kind, "已准备完成"),
		}.withResource(resource, report.ArchivePath, report.StoreRoot))
		return report, nil
	}
	if report, ok := m.prepareSystemChromiumIfAvailable(ctx, kind, report, resource, options.Progress); ok {
		return report, nil
	}

	lockPath := LockPath(m.repoRoot)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, m.classifyBootstrapErrorWithProgress(options.Progress, kind, resource, "lock", "", nil, fmt.Errorf("create deps lock root: %w", err))
	}
	emitPrepareProgress(options.Progress, PrepareProgress{
		Stage:   "lock",
		Status:  "running",
		Summary: "正在等待 " + managedResourceLabel(kind) + "准备锁",
	}.withResource(resource, report.ArchivePath, report.StoreRoot))
	release, err := acquireLock(ctx, lockPath, m.now)
	if err != nil {
		return nil, m.classifyBootstrapErrorWithProgress(options.Progress, kind, resource, "lock", "", nil, err)
	}
	defer release()

	if prepared, err := m.resolvePreparedManifestResource(manifest, resource); err == nil {
		report.UsedPreparedStore = true
		report.Entrypoints = prepared.Entrypoints
		report.PreparedEntrypoint = primaryEntrypoint(prepared)
		emitPrepareProgress(options.Progress, PrepareProgress{
			Stage:    "complete",
			Status:   "succeeded",
			Progress: 100,
			Summary:  managedResourceLabel(kind) + "已准备完成",
		}.withResource(resource, report.ArchivePath, report.StoreRoot))
		return report, nil
	}
	if report, ok := m.prepareSystemChromiumIfAvailable(ctx, kind, report, resource, options.Progress); ok {
		return report, nil
	}

	if err := os.MkdirAll(CacheRoot(m.repoRoot), 0o755); err != nil {
		return nil, m.classifyBootstrapErrorWithProgress(options.Progress, kind, resource, "download", "", nil, fmt.Errorf("create deps cache root: %w", err))
	}
	if verifyFileSHA256(report.ArchivePath, resource.SHA256) == nil {
		report.UsedCachedArchive = true
		emitPrepareProgress(options.Progress, PrepareProgress{
			Stage:    "download",
			Status:   "succeeded",
			Progress: 100,
			Summary:  managedResourceLabel(kind) + "安装包已下载",
		}.withResource(resource, report.ArchivePath, report.StoreRoot))
	}
	sourceSelector := m.selectSources
	if m.downloadFile != nil && !sameFunction(m.downloadFile, downloadHTTPSFile) && sameFunction(sourceSelector, selectDownloadSources) {
		sourceSelector = nil
	}
	selectedSource, attemptedSources, err := ensureDownloadedArchiveWithProgress(ctx, report.ArchivePath, report.StoreRoot, resource, m.downloadFile, sourceSelector, options.Progress)
	report.SelectedSource = strings.TrimSpace(selectedSource)
	report.AttemptedSources = append(report.AttemptedSources, attemptedSources...)
	if err != nil {
		stage := "download"
		if strings.Contains(err.Error(), "verify deps resource") || strings.Contains(err.Error(), "persist deps archive") {
			stage = "verify"
		}
		return nil, m.classifyBootstrapErrorWithProgress(options.Progress, kind, resource, stage, report.SelectedSource, report.AttemptedSources, err)
	}
	if err := ensurePreparedResourceWithProgress(ctx, m.repoRoot, *resource, report.ArchivePath, m.extract, options.Progress); err != nil {
		return nil, m.classifyBootstrapErrorWithProgress(options.Progress, kind, resource, "extract", report.SelectedSource, report.AttemptedSources, err)
	}

	prepared, err := m.resolvePreparedManifestResource(manifest, resource)
	if err != nil {
		return nil, m.classifyBootstrapErrorWithProgress(options.Progress, kind, resource, "entrypoint", report.SelectedSource, report.AttemptedSources, err)
	}
	report.Entrypoints = prepared.Entrypoints
	report.PreparedEntrypoint = primaryEntrypoint(prepared)
	emitPrepareProgress(options.Progress, PrepareProgress{
		Stage:    "complete",
		Status:   "succeeded",
		Progress: 100,
		Summary:  managedResourceText(kind, "已准备完成"),
	}.withResource(resource, report.ArchivePath, report.StoreRoot))
	return report, nil
}

func (m *Manager) prepareSystemChromiumIfAvailable(ctx context.Context, kind string, report *PrepareReport, resource *Resource, reporter PrepareProgressReporter) (*PrepareReport, bool) {
	if kind != "chromium" {
		return nil, false
	}
	path, err := m.resolveSystemChromiumEntrypoint(ctx)
	if err != nil {
		return nil, false
	}
	if report == nil {
		report = systemChromiumPrepareReport(path)
	} else {
		report.UsedSystemBrowser = true
		report.PreparedEntrypoint = path
		report.Entrypoints = map[string]string{"browser": path}
	}
	emitPrepareProgress(reporter, PrepareProgress{
		Kind:     kind,
		Label:    managedResourceLabel(kind),
		Stage:    "complete",
		Status:   "succeeded",
		Progress: 100,
		Summary:  managedResourceText(kind, "已准备完成"),
	}.withResource(resource, report.ArchivePath, report.StoreRoot))
	return report, true
}

func systemChromiumPrepareReport(path string) *PrepareReport {
	prepared := systemChromiumPreparedResource(path)
	if prepared == nil {
		return &PrepareReport{Kind: "chromium"}
	}
	return &PrepareReport{
		Kind:               "chromium",
		Resource:           prepared.Resource,
		StoreRoot:          prepared.Root,
		UsedSystemBrowser:  true,
		PreparedEntrypoint: path,
		Entrypoints:        prepared.Entrypoints,
	}
}
