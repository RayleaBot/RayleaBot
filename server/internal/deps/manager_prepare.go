package deps

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Manager struct {
	repoRoot           string
	downloadFile       func(context.Context, string, string) error
	selectSources      func(context.Context, []ResourceSource) []ResourceSource
	extract            func(context.Context, string, string, string) error
	findSystemChromium func(context.Context) (string, error)
	now                func() time.Time
}

var systemChromiumFinder = FindSystemChromium

func SetSystemChromiumFinderForTest(finder func(context.Context) (string, error)) func() {
	previous := systemChromiumFinder
	systemChromiumFinder = finder
	return func() {
		systemChromiumFinder = previous
	}
}

func NewManager(repoRoot string) *Manager {
	return &Manager{
		repoRoot:           strings.TrimSpace(repoRoot),
		findSystemChromium: systemChromiumFinder,
		now:                time.Now,
	}
}

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
	if sourceSelector == nil && m.downloadFile == nil {
		sourceSelector = SelectSources
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

func (m *Manager) Inspect(kind string) (*BootstrapInspection, error) {
	if m == nil {
		return nil, errors.New("deps manager is required")
	}

	manifest, resource, err := m.currentResource(kind)
	if err != nil {
		if kind == "chromium" {
			if path, pathErr := m.resolveSystemChromiumEntrypoint(context.Background()); pathErr == nil {
				return &BootstrapInspection{
					Kind:                 kind,
					MetadataComplete:     true,
					PreparedStorePresent: true,
					SystemBrowserPath:    path,
				}, nil
			}
		}
		return nil, classifyBootstrapError(m.repoRoot, kind, nil, "manifest", "", nil, err)
	}
	inspection := &BootstrapInspection{
		Kind:             kind,
		Resource:         resource,
		ArchivePath:      filepath.Join(CacheRoot(m.repoRoot), resource.ID+"-"+resource.Version+archiveSuffix(resource.ArchiveFormat)),
		StoreRoot:        StoreRoot(m.repoRoot, resource),
		MetadataComplete: manifest.HasPlatform(CurrentPlatform()) && ResourceMetadataComplete(resource),
	}
	if !inspection.MetadataComplete && kind == "chromium" {
		if path, err := m.resolveSystemChromiumEntrypoint(context.Background()); err == nil {
			inspection.MetadataComplete = true
			inspection.PreparedStorePresent = true
			inspection.SystemBrowserPath = path
			return inspection, nil
		}
	}
	if inspection.MetadataComplete && verifyFileSHA256(inspection.ArchivePath, resource.SHA256) == nil {
		inspection.CachedArchivePresent = true
	}
	if _, err := m.resolvePreparedManifestResource(manifest, resource); err == nil {
		inspection.PreparedStorePresent = true
		return inspection, nil
	}
	if kind == "chromium" {
		if path, err := m.resolveSystemChromiumEntrypoint(context.Background()); err == nil {
			inspection.PreparedStorePresent = true
			inspection.SystemBrowserPath = path
		}
	}
	return inspection, nil
}

func (m *Manager) ResolvePreparedEntrypoint(kind, name string) (string, error) {
	prepared, err := m.resolvePreparedResource(kind)
	if err != nil {
		if kind == "chromium" && name == "browser" {
			return m.resolveSystemChromiumEntrypoint(context.Background())
		}
		return "", err
	}
	path, ok := prepared.Entrypoints[name]
	if !ok {
		return "", fmt.Errorf("entrypoint %s is not declared for %s", name, kind)
	}
	return path, nil
}

func (m *Manager) ResolveEntrypoint(ctx context.Context, kind, name string) (string, error) {
	prepared, err := m.Prepare(ctx, kind)
	if err != nil {
		return "", err
	}
	path, ok := prepared.Entrypoints[name]
	if !ok {
		return "", fmt.Errorf("entrypoint %s is not declared for %s", name, kind)
	}
	return path, nil
}

func (m *Manager) resolveSystemChromiumEntrypoint(ctx context.Context) (string, error) {
	if m == nil || m.findSystemChromium == nil {
		return "", errSystemChromiumUnavailable
	}
	path, err := m.findSystemChromium(ctx)
	if err != nil {
		return "", err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errSystemChromiumUnavailable
	}
	return path, nil
}

func systemChromiumPreparedResource(path string) *PreparedResource {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	resource := Resource{
		ID:       "system-chromium",
		Kind:     "chromium",
		Version:  "system",
		Platform: CurrentPlatform(),
	}
	return &PreparedResource{
		Resource: resource,
		Root:     filepath.Dir(path),
		Entrypoints: map[string]string{
			"browser": path,
		},
	}
}

func (m *Manager) resolvePreparedResource(kind string) (*PreparedResource, error) {
	manifest, resource, err := m.currentResource(kind)
	if err != nil {
		if kind == "chromium" {
			if path, pathErr := m.resolveSystemChromiumEntrypoint(context.Background()); pathErr == nil {
				return systemChromiumPreparedResource(path), nil
			}
		}
		return nil, err
	}
	prepared, err := m.resolvePreparedManifestResource(manifest, resource)
	if err == nil {
		return prepared, nil
	}
	if kind == "chromium" {
		if path, pathErr := m.resolveSystemChromiumEntrypoint(context.Background()); pathErr == nil {
			return systemChromiumPreparedResource(path), nil
		}
	}
	return nil, err
}

func (m *Manager) resolvePreparedManifestResource(_ *Manifest, resource *Resource) (*PreparedResource, error) {
	storeRoot := StoreRoot(m.repoRoot, resource)
	entrypoints, err := resolvePreparedEntrypoints(storeRoot, resource)
	if err != nil {
		return nil, err
	}
	return &PreparedResource{
		Resource:    *resource,
		Root:        storeRoot,
		Entrypoints: entrypoints,
	}, nil
}

func (m *Manager) currentResource(kind string) (*Manifest, *Resource, error) {
	manifest, err := LoadManifest(m.repoRoot)
	if err != nil {
		return nil, nil, err
	}
	resource := manifest.FindResource(CurrentPlatform(), kind)
	if resource == nil {
		return manifest, nil, fmt.Errorf("deps manifest does not include %s for %s", kind, CurrentPlatform())
	}
	return manifest, resource, nil
}

func resolvePreparedEntrypoints(storeRoot string, resource *Resource) (map[string]string, error) {
	if resource == nil {
		return nil, errors.New("deps resource is required")
	}
	entrypoints := make(map[string]string, len(resource.Entrypoints))
	for _, key := range requiredEntrypoints(resource) {
		candidates := resource.Entrypoints[key]
		var resolved string
		for _, candidate := range candidates {
			clean := filepath.Clean(filepath.Join(storeRoot, filepath.FromSlash(candidate)))
			if !pathWithinRoot(storeRoot, clean) {
				continue
			}
			info, err := os.Stat(clean)
			if err != nil || info.IsDir() {
				continue
			}
			resolved = clean
			break
		}
		if resolved == "" {
			return nil, fmt.Errorf("prepared deps resource %s is missing entrypoint %s", resource.Kind, key)
		}
		entrypoints[key] = resolved
	}
	return entrypoints, nil
}

func primaryEntrypoint(prepared *PreparedResource) string {
	if prepared == nil {
		return ""
	}
	for _, key := range requiredEntrypoints(&prepared.Resource) {
		if entry := strings.TrimSpace(prepared.Entrypoints[key]); entry != "" {
			return entry
		}
	}
	return ""
}
