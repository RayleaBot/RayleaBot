package deps

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const ManifestVersion = 3

const (
	sourceProbeBytes          int64 = 1024 * 1024
	sourceProbePerSourceLimit       = 8 * time.Second
	sourceProbeOverallLimit         = 12 * time.Second
	sourceProbeCloseRatio           = 0.10
)

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Manifest struct {
	ManifestVersion int        `json:"manifest_version"`
	Resources       []Resource `json:"resources"`
}

type ResourceSource struct {
	URL   string `json:"url"`
	Kind  string `json:"kind"`
	Label string `json:"label,omitempty"`
}

type Resource struct {
	ID            string              `json:"id"`
	Kind          string              `json:"kind"`
	Version       string              `json:"version"`
	Platform      string              `json:"platform"`
	Sources       []ResourceSource    `json:"sources"`
	SHA256        string              `json:"sha256"`
	ArchiveFormat string              `json:"archive_format"`
	Entrypoints   map[string][]string `json:"entrypoints"`
}

type PreparedResource struct {
	Resource    Resource
	Root        string
	Entrypoints map[string]string
}

type BootstrapInspection struct {
	Kind                 string
	Resource             *Resource
	ArchivePath          string
	StoreRoot            string
	MetadataComplete     bool
	CachedArchivePresent bool
	PreparedStorePresent bool
}

type PrepareReport struct {
	Kind               string
	Resource           Resource
	ArchivePath        string
	StoreRoot          string
	UsedPreparedStore  bool
	UsedCachedArchive  bool
	AttemptedSources   []string
	SelectedSource     string
	PreparedEntrypoint string
	Entrypoints        map[string]string
}

type BootstrapError struct {
	Kind             string
	Stage            string
	SelectedSource   string
	AttemptedSources []string
	ArchivePath      string
	StoreRoot        string
	Remediation      string
	Message          string
	Err              error
}

func (e *BootstrapError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "managed runtime bootstrap failed"
}

func (e *BootstrapError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *BootstrapError) Details() map[string]any {
	if e == nil {
		return nil
	}
	details := map[string]any{
		"resource_kind": e.Kind,
		"stage":         e.Stage,
	}
	if strings.TrimSpace(e.SelectedSource) != "" {
		details["selected_source"] = e.SelectedSource
	}
	if len(e.AttemptedSources) > 0 {
		details["attempted_sources"] = append([]string(nil), e.AttemptedSources...)
	}
	if strings.TrimSpace(e.ArchivePath) != "" {
		details["archive_path"] = e.ArchivePath
	}
	if strings.TrimSpace(e.StoreRoot) != "" {
		details["store_root"] = e.StoreRoot
	}
	if strings.TrimSpace(e.Remediation) != "" {
		details["remediation"] = e.Remediation
	}
	return details
}

func ManagedResourceLabel(kind string) string {
	return managedResourceLabel(kind)
}

func BootstrapRemediation(kind, archivePath, storeRoot string) string {
	return bootstrapRemediation(kind, archivePath, storeRoot)
}

func BootstrapSummary(kind string, inspection *BootstrapInspection) string {
	label := managedResourceLabel(kind)
	switch {
	case inspection == nil:
		return label + "清单不可用。"
	case !inspection.MetadataComplete:
		return label + "元数据不完整。"
	case inspection.PreparedStorePresent:
		return label + "已准备完成。"
	case inspection.CachedArchivePresent:
		if kind == "python-runtime" || kind == "nodejs-runtime" {
			return label + "已下载，启动时会解压。"
		}
		return label + "已下载，未解压。"
	default:
		if kind == "python-runtime" || kind == "nodejs-runtime" {
			return label + "已纳入启动流程。"
		}
		return label + "未准备。"
	}
}

type Manager struct {
	repoRoot      string
	downloadFile  func(context.Context, string, string) error
	selectSources func(context.Context, []ResourceSource) []ResourceSource
	extract       func(context.Context, string, string, string) error
	now           func() time.Time
}

func NewManager(repoRoot string) *Manager {
	return &Manager{
		repoRoot:      strings.TrimSpace(repoRoot),
		downloadFile:  downloadHTTPSFile,
		selectSources: selectDownloadSources,
		extract:       extractArchive,
		now:           time.Now,
	}
}

func LoadManifest(repoRoot string) (*Manifest, error) {
	return LoadManifestPath(filepath.Join(strings.TrimSpace(repoRoot), ".deps", "manifest.json"))
}

func LoadManifestPath(manifestPath string) (*Manifest, error) {
	payload, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return nil, fmt.Errorf("decode deps manifest: %w", err)
	}
	if manifest.ManifestVersion != ManifestVersion {
		return nil, fmt.Errorf("unsupported deps manifest version %d", manifest.ManifestVersion)
	}
	return &manifest, nil
}

func CurrentPlatform() string {
	return ManifestPlatform(runtime.GOOS, runtime.GOARCH)
}

func ManifestPlatform(goos, goarch string) string {
	switch goos {
	case "windows":
		return "windows-" + normalizeManifestArch(goarch)
	case "darwin":
		return "macos-" + normalizeManifestArch(goarch)
	default:
		return goos + "-" + normalizeManifestArch(goarch)
	}
}

func normalizeManifestArch(goarch string) string {
	switch goarch {
	case "amd64":
		return "x64"
	default:
		return goarch
	}
}

func (manifest *Manifest) HasPlatform(platform string) bool {
	if manifest == nil {
		return false
	}
	for _, resource := range manifest.Resources {
		if resource.Platform == platform {
			return true
		}
	}
	return false
}

func (manifest *Manifest) FindResource(platform, kind string) *Resource {
	if manifest == nil {
		return nil
	}
	for i := range manifest.Resources {
		resource := &manifest.Resources[i]
		if resource.Platform == platform && resource.Kind == kind {
			return resource
		}
	}
	return nil
}

func ResourceMetadataComplete(resource *Resource) bool {
	if resource == nil {
		return false
	}
	if strings.TrimSpace(resource.ArchiveFormat) == "" {
		return false
	}
	if !archiveFormatSupported(resource.ArchiveFormat) {
		return false
	}
	if !resourceHasRequiredEntrypoints(resource) {
		return false
	}
	if !resourceSourcesComplete(resource) {
		return false
	}
	sha256 := strings.ToLower(strings.TrimSpace(resource.SHA256))
	if strings.Contains(strings.ToUpper(sha256), "TODO(") {
		return false
	}
	return sha256Pattern.MatchString(sha256)
}

func resourceSourcesComplete(resource *Resource) bool {
	if resource == nil || len(resource.Sources) == 0 {
		return false
	}
	seen := map[string]struct{}{}
	for _, source := range resource.Sources {
		rawURL := strings.TrimSpace(source.URL)
		if rawURL == "" || strings.Contains(strings.ToUpper(rawURL), "TODO(") {
			return false
		}
		parsedURL, err := url.Parse(rawURL)
		if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
			return false
		}
		if !validResourceSourceKind(strings.TrimSpace(source.Kind)) {
			return false
		}
		if _, ok := seen[rawURL]; ok {
			return false
		}
		seen[rawURL] = struct{}{}
	}
	return true
}

func validResourceSourceKind(kind string) bool {
	switch kind {
	case "upstream", "mirror":
		return true
	default:
		return false
	}
}

func archiveFormatSupported(format string) bool {
	switch strings.TrimSpace(format) {
	case "zip", "tar.gz", "tar.xz":
		return true
	default:
		return false
	}
}

func resourceHasRequiredEntrypoints(resource *Resource) bool {
	required := requiredEntrypoints(resource)
	if len(required) == 0 {
		return false
	}
	if len(resource.Entrypoints) == 0 {
		return false
	}
	for _, key := range required {
		candidates := resource.Entrypoints[key]
		if len(candidates) == 0 {
			return false
		}
		valid := false
		for _, candidate := range candidates {
			clean := strings.TrimSpace(candidate)
			if clean == "" {
				continue
			}
			if filepath.IsAbs(clean) {
				continue
			}
			if clean == "." || strings.HasPrefix(clean, "..") {
				continue
			}
			valid = true
			break
		}
		if !valid {
			return false
		}
	}
	return true
}

func requiredEntrypoints(resource *Resource) []string {
	if resource == nil {
		return nil
	}
	switch resource.Kind {
	case "chromium":
		return []string{"browser"}
	case "python-runtime":
		return []string{"python"}
	case "nodejs-runtime":
		return []string{"node", "npm"}
	default:
		return nil
	}
}

func StoreRoot(repoRoot string, resource *Resource) string {
	if resource == nil {
		return ""
	}
	return filepath.Join(strings.TrimSpace(repoRoot), ".deps", "store", resource.ID, resource.Version)
}

func CacheRoot(repoRoot string) string {
	return filepath.Join(strings.TrimSpace(repoRoot), "cache", "downloads", "runtime")
}

func LockPath(repoRoot string) string {
	return filepath.Join(strings.TrimSpace(repoRoot), "cache", "downloads", "platform.lock")
}

func (m *Manager) ResolvePreparedEntrypoint(kind, name string) (string, error) {
	prepared, err := m.resolvePreparedResource(kind)
	if err != nil {
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

func (m *Manager) Inspect(kind string) (*BootstrapInspection, error) {
	if m == nil {
		return nil, errors.New("deps manager is required")
	}

	manifest, resource, err := m.currentResource(kind)
	if err != nil {
		return nil, classifyBootstrapError(m.repoRoot, kind, nil, "manifest", "", nil, err)
	}
	inspection := &BootstrapInspection{
		Kind:             kind,
		Resource:         resource,
		ArchivePath:      filepath.Join(CacheRoot(m.repoRoot), resource.ID+"-"+resource.Version+archiveSuffix(resource.ArchiveFormat)),
		StoreRoot:        StoreRoot(m.repoRoot, resource),
		MetadataComplete: manifest.HasPlatform(CurrentPlatform()) && ResourceMetadataComplete(resource),
	}
	if inspection.MetadataComplete && verifyFileSHA256(inspection.ArchivePath, resource.SHA256) == nil {
		inspection.CachedArchivePresent = true
	}
	if _, err := m.resolvePreparedManifestResource(manifest, resource); err == nil {
		inspection.PreparedStorePresent = true
	}
	return inspection, nil
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
		return nil, m.classifyBootstrapErrorWithProgress(options.Progress, kind, resource, "manifest", "", nil, fmt.Errorf("deps manifest does not include current platform %s", CurrentPlatform()))
	}
	if !ResourceMetadataComplete(resource) {
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
			Summary:  managedResourceLabel(kind) + "已准备完成",
		}.withResource(resource, report.ArchivePath, report.StoreRoot))
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
		Summary:  managedResourceLabel(kind) + "已准备完成",
	}.withResource(resource, report.ArchivePath, report.StoreRoot))
	return report, nil
}

func (m *Manager) resolvePreparedResource(kind string) (*PreparedResource, error) {
	manifest, resource, err := m.currentResource(kind)
	if err != nil {
		return nil, err
	}
	return m.resolvePreparedManifestResource(manifest, resource)
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

func classifyBootstrapError(repoRoot, kind string, resource *Resource, stage string, selectedSource string, attemptedSources []string, err error) error {
	if err == nil {
		return nil
	}
	archivePath := ""
	storeRoot := ""
	if resource != nil {
		archivePath = filepath.Join(CacheRoot(repoRoot), resource.ID+"-"+resource.Version+archiveSuffix(resource.ArchiveFormat))
		storeRoot = StoreRoot(repoRoot, resource)
	}
	return &BootstrapError{
		Kind:             kind,
		Stage:            stage,
		SelectedSource:   strings.TrimSpace(selectedSource),
		AttemptedSources: append([]string(nil), attemptedSources...),
		ArchivePath:      archivePath,
		StoreRoot:        storeRoot,
		Remediation:      bootstrapRemediation(kind, archivePath, storeRoot),
		Message:          bootstrapMessage(kind, stage),
		Err:              err,
	}
}

func (m *Manager) classifyBootstrapErrorWithProgress(reporter PrepareProgressReporter, kind string, resource *Resource, stage string, selectedSource string, attemptedSources []string, err error) error {
	bootstrapErr := classifyBootstrapError(m.repoRoot, kind, resource, stage, selectedSource, attemptedSources, err)
	if bootstrapErr == nil {
		return nil
	}
	var details *BootstrapError
	if errors.As(bootstrapErr, &details) {
		sourceURL := strings.TrimSpace(selectedSource)
		if sourceURL == "" && len(attemptedSources) > 0 {
			sourceURL = strings.TrimSpace(attemptedSources[len(attemptedSources)-1])
		}
		emitPrepareProgress(reporter, PrepareProgress{
			Kind:        kind,
			Stage:       stage,
			Status:      "failed",
			SourceURL:   sourceURL,
			ArchivePath: details.ArchivePath,
			StoreRoot:   details.StoreRoot,
			Summary:     details.Message,
			Error:       err.Error(),
		}.withResource(resource, details.ArchivePath, details.StoreRoot))
	}
	return bootstrapErr
}

func bootstrapMessage(kind, stage string) string {
	resourceLabel := managedResourceLabel(kind)
	switch stage {
	case "manifest":
		return resourceLabel + "清单不可用"
	case "lock":
		return resourceLabel + "准备锁等待超时"
	case "download":
		return resourceLabel + "安装包下载失败"
	case "verify":
		return resourceLabel + "安装包校验失败"
	case "extract":
		return resourceLabel + "安装包解压失败"
	case "entrypoint":
		return resourceLabel + "入口文件缺失"
	default:
		return resourceLabel + "准备失败"
	}
}

func bootstrapRemediation(kind, archivePath, storeRoot string) string {
	paths := []string{}
	if strings.TrimSpace(archivePath) != "" {
		paths = append(paths, "下载位置："+archivePath+"。")
	}
	if strings.TrimSpace(storeRoot) != "" {
		paths = append(paths, "解压位置："+storeRoot+"。")
	}
	locationText := strings.Join(paths, "")
	switch kind {
	case "chromium":
		return "启动运行环境任务准备 Chromium 浏览环境，或在配置中设置 render.browser_path。" + locationText
	case "python-runtime":
		return "启动运行环境任务准备 Python 运行环境。" + locationText
	case "nodejs-runtime":
		return "启动运行环境任务准备 Node.js 和 npm 环境。" + locationText
	default:
		return "启动运行环境任务准备依赖。" + locationText
	}
}

func managedResourceLabel(kind string) string {
	switch kind {
	case "chromium":
		return "Chromium 浏览环境"
	case "python-runtime":
		return "Python 运行环境"
	case "nodejs-runtime":
		return "Node.js / npm 环境"
	default:
		return "运行环境"
	}
}

func pathWithinRoot(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func ensureDownloadedArchive(ctx context.Context, archivePath string, resource *Resource, downloader func(context.Context, string, string) error) (string, []string, error) {
	sourceSelector := selectDownloadSources
	if downloader != nil && !sameFunction(downloader, downloadHTTPSFile) {
		sourceSelector = nil
	}
	return ensureDownloadedArchiveWithProgress(ctx, archivePath, StoreRoot("", resource), resource, downloader, sourceSelector, nil)
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
			Summary:  managedResourceLabel(resource.Kind) + "安装包已下载",
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
			Summary:  "正在测试 " + managedResourceLabel(resource.Kind) + "下载来源",
			Progress: 0,
		}.withResource(resource, archivePath, storeRoot))
		selectedSources := sourceSelector(ctx, downloadSources)
		if len(selectedSources) > 0 {
			downloadSources = selectedSources
		}
		emitPrepareProgress(reporter, PrepareProgress{
			Stage:    "probe",
			Status:   "succeeded",
			Summary:  managedResourceLabel(resource.Kind) + "下载来源已测试",
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
			Summary:     "正在校验 " + managedResourceLabel(resource.Kind) + "安装包",
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
			Summary:     managedResourceLabel(resource.Kind) + "安装包已下载",
		}.withResource(resource, archivePath, storeRoot))
		return rawURL, attempted, nil
	}
	if finalErr == nil {
		finalErr = fmt.Errorf("download deps resource %s: no usable source configured", resource.Kind)
	}
	return "", attempted, finalErr
}

func normalizedResourceSources(sources []ResourceSource) []ResourceSource {
	normalized := make([]ResourceSource, 0, len(sources))
	for _, source := range sources {
		if strings.TrimSpace(source.URL) == "" {
			continue
		}
		source.URL = strings.TrimSpace(source.URL)
		source.Label = strings.TrimSpace(source.Label)
		source.Kind = strings.TrimSpace(source.Kind)
		normalized = append(normalized, source)
	}
	return normalized
}

func downloadSourceSummary(kind string, source ResourceSource) string {
	label := strings.TrimSpace(source.Label)
	if label == "" {
		return "正在下载 " + managedResourceLabel(kind)
	}
	return "正在从 " + label + " 下载 " + managedResourceLabel(kind)
}

type sourceProbeResult struct {
	source      ResourceSource
	index       int
	bytesRead   int64
	duration    time.Duration
	bytesPerSec float64
	ok          bool
}

func selectDownloadSources(ctx context.Context, sources []ResourceSource) []ResourceSource {
	normalized := normalizedResourceSources(sources)
	if len(normalized) <= 1 {
		return normalized
	}
	ctx, cancel := context.WithTimeout(ctx, sourceProbeOverallLimit)
	defer cancel()

	results := make([]sourceProbeResult, len(normalized))
	var wg sync.WaitGroup
	for index, source := range normalized {
		results[index] = sourceProbeResult{source: source, index: index}
		wg.Add(1)
		go func(index int, source ResourceSource) {
			defer wg.Done()
			result := probeDownloadSource(ctx, source, index)
			results[index] = result
		}(index, source)
	}
	wg.Wait()

	successful := make([]sourceProbeResult, 0, len(results))
	failed := make([]sourceProbeResult, 0, len(results))
	for _, result := range results {
		if result.ok {
			successful = append(successful, result)
			continue
		}
		failed = append(failed, result)
	}
	if len(successful) == 0 {
		return normalized
	}
	sort.SliceStable(successful, func(i, j int) bool {
		if successful[i].bytesPerSec == successful[j].bytesPerSec {
			return successful[i].index < successful[j].index
		}
		return successful[i].bytesPerSec > successful[j].bytesPerSec
	})
	sort.SliceStable(failed, func(i, j int) bool {
		return failed[i].index < failed[j].index
	})
	ordered := restoreCloseProbeOrder(successful)
	ordered = append(ordered, failed...)
	selected := make([]ResourceSource, 0, len(ordered))
	for _, result := range ordered {
		selected = append(selected, result.source)
	}
	return selected
}

func restoreCloseProbeOrder(results []sourceProbeResult) []sourceProbeResult {
	if len(results) <= 1 {
		return append([]sourceProbeResult(nil), results...)
	}
	ordered := make([]sourceProbeResult, 0, len(results))
	group := make([]sourceProbeResult, 0, len(results))
	groupBest := 0.0
	flush := func() {
		sort.SliceStable(group, func(i, j int) bool {
			return group[i].index < group[j].index
		})
		ordered = append(ordered, group...)
		group = nil
	}
	for _, result := range results {
		if len(group) == 0 {
			group = append(group, result)
			groupBest = result.bytesPerSec
			continue
		}
		if groupBest > 0 && math.Abs(groupBest-result.bytesPerSec)/groupBest <= sourceProbeCloseRatio {
			group = append(group, result)
			continue
		}
		flush()
		group = append(group, result)
		groupBest = result.bytesPerSec
	}
	if len(group) > 0 {
		flush()
	}
	return ordered
}

func probeDownloadSource(ctx context.Context, source ResourceSource, index int) sourceProbeResult {
	result := sourceProbeResult{source: source, index: index}
	probeCtx, cancel := context.WithTimeout(ctx, sourceProbePerSourceLimit)
	defer cancel()

	request, err := http.NewRequestWithContext(probeCtx, http.MethodGet, source.URL, nil)
	if err != nil {
		return result
	}
	request.Header.Set("Range", fmt.Sprintf("bytes=0-%d", sourceProbeBytes-1))
	startedAt := time.Now()
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return result
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		return result
	}
	limited := io.LimitReader(response.Body, sourceProbeBytes)
	bytesRead, err := io.Copy(io.Discard, limited)
	if err != nil || bytesRead <= 0 {
		return result
	}
	duration := time.Since(startedAt)
	if duration <= 0 {
		duration = time.Millisecond
	}
	result.bytesRead = bytesRead
	result.duration = duration
	result.bytesPerSec = float64(bytesRead) / duration.Seconds()
	result.ok = true
	return result
}

func ensurePreparedResource(
	ctx context.Context,
	repoRoot string,
	resource Resource,
	archivePath string,
	extractor func(context.Context, string, string, string) error,
) error {
	return ensurePreparedResourceWithProgress(ctx, repoRoot, resource, archivePath, extractor, nil)
}

func ensurePreparedResourceWithProgress(
	ctx context.Context,
	repoRoot string,
	resource Resource,
	archivePath string,
	extractor func(context.Context, string, string, string) error,
	reporter PrepareProgressReporter,
) error {
	storeRoot := StoreRoot(repoRoot, &resource)
	if _, err := resolvePreparedEntrypoints(storeRoot, &resource); err == nil {
		emitPrepareProgress(reporter, PrepareProgress{
			Stage:    "extract",
			Status:   "succeeded",
			Progress: 100,
			Summary:  managedResourceLabel(resource.Kind) + "已解压",
		}.withResource(&resource, archivePath, storeRoot))
		return nil
	} else if _, statErr := os.Stat(storeRoot); statErr == nil {
		emitPrepareProgress(reporter, PrepareProgress{
			Stage:   "cleanup",
			Status:  "running",
			Summary: "正在清理未完成的 " + managedResourceLabel(resource.Kind) + "目录",
		}.withResource(&resource, archivePath, storeRoot))
		if removeErr := os.RemoveAll(storeRoot); removeErr != nil {
			return fmt.Errorf("clean incomplete deps store root: %w", removeErr)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("inspect deps store root: %w", statErr)
	}
	if err := os.MkdirAll(filepath.Dir(storeRoot), 0o755); err != nil {
		return fmt.Errorf("create deps store root: %w", err)
	}
	if err := removeStaleTempRoots(filepath.Dir(storeRoot), resource.ID, resource.Version); err != nil {
		return fmt.Errorf("clean stale deps temp roots: %w", err)
	}
	tempRoot, err := os.MkdirTemp(filepath.Dir(storeRoot), "."+resource.ID+"-"+resource.Version+"-*")
	if err != nil {
		return fmt.Errorf("create deps temp root: %w", err)
	}
	defer os.RemoveAll(tempRoot)

	emitPrepareProgress(reporter, PrepareProgress{
		Stage:   "extract",
		Status:  "running",
		Summary: "正在解压 " + managedResourceLabel(resource.Kind),
	}.withResource(&resource, archivePath, storeRoot))
	if err := extractWithProgress(ctx, archivePath, resource.ArchiveFormat, tempRoot, extractor, func(progress extractProgress) {
		emitPrepareProgress(reporter, PrepareProgress{
			Stage:            "extract",
			Status:           "running",
			Progress:         progress.Progress,
			ExtractedEntries: progress.ExtractedEntries,
			TotalEntries:     progress.TotalEntries,
			Summary:          "正在解压 " + managedResourceLabel(resource.Kind),
		}.withResource(&resource, archivePath, storeRoot))
	}); err != nil {
		return fmt.Errorf("extract deps resource %s: %w", resource.Kind, err)
	}
	emitPrepareProgress(reporter, PrepareProgress{
		Stage:    "extract",
		Status:   "succeeded",
		Progress: 100,
		Summary:  managedResourceLabel(resource.Kind) + "已解压",
	}.withResource(&resource, archivePath, storeRoot))
	emitPrepareProgress(reporter, PrepareProgress{
		Stage:   "activate",
		Status:  "running",
		Summary: "正在启用 " + managedResourceLabel(resource.Kind),
	}.withResource(&resource, archivePath, storeRoot))
	_ = os.RemoveAll(storeRoot)
	if err := os.Rename(tempRoot, storeRoot); err != nil {
		return fmt.Errorf("activate deps resource %s: %w", resource.Kind, err)
	}
	emitPrepareProgress(reporter, PrepareProgress{
		Stage:    "activate",
		Status:   "succeeded",
		Progress: 100,
		Summary:  managedResourceLabel(resource.Kind) + "已启用",
	}.withResource(&resource, archivePath, storeRoot))
	return nil
}

func removeStaleTempRoots(parent, resourceID, version string) error {
	entries, err := os.ReadDir(parent)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	prefix := "." + resourceID + "-" + version + "-"
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(parent, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func archiveSuffix(format string) string {
	switch format {
	case "tar.gz":
		return ".tar.gz"
	case "tar.xz":
		return ".tar.xz"
	default:
		return ".zip"
	}
}

func verifyFileSHA256(path string, want string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}
	got := hex.EncodeToString(hasher.Sum(nil))
	if strings.ToLower(strings.TrimSpace(want)) != got {
		return fmt.Errorf("sha256 mismatch: got %s want %s", got, want)
	}
	return nil
}

func acquireLock(ctx context.Context, path string, now func() time.Time) (func(), error) {
	for {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = io.WriteString(file, fmt.Sprintf("%d %s\n", os.Getpid(), now().UTC().Format(time.RFC3339)))
			_ = file.Close()
			return func() {
				_ = os.Remove(path)
			}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("acquire deps lock: %w", err)
		}
		info, statErr := os.Stat(path)
		if statErr == nil && now().Sub(info.ModTime()) > 30*time.Minute {
			_ = os.Remove(path)
			continue
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

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

func extractArchive(ctx context.Context, archivePath, archiveFormat, destRoot string) error {
	return extractWithProgress(ctx, archivePath, archiveFormat, destRoot, nil, nil)
}

func extractWithProgress(ctx context.Context, archivePath, archiveFormat, destRoot string, extractor func(context.Context, string, string, string) error, progress func(extractProgress)) error {
	if extractor != nil && reflect.ValueOf(extractor).Pointer() != reflect.ValueOf(extractArchive).Pointer() {
		return extractor(ctx, archivePath, archiveFormat, destRoot)
	}
	switch archiveFormat {
	case "zip":
		return extractZipWithProgress(archivePath, destRoot, progress)
	case "tar.gz":
		return extractTarGzWithProgress(archivePath, destRoot, progress)
	case "tar.xz":
		if progress != nil {
			progress(extractProgress{Progress: 0})
		}
		cmd := exec.CommandContext(ctx, "tar", "-xf", archivePath, "-C", destRoot)
		output, err := cmd.CombinedOutput()
		if err != nil {
			if len(output) == 0 {
				return err
			}
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
		}
		if progress != nil {
			progress(extractProgress{Progress: 100})
		}
		return nil
	default:
		return fmt.Errorf("unsupported archive format %s", archiveFormat)
	}
}

type progressReader struct {
	reader     io.Reader
	total      int64
	read       int64
	lastNotify int
	lastBytes  int64
	notify     func(downloadProgress)
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
	percent := prepareProgressPercent(r.read, r.total)
	if !force && r.total <= 0 && r.read-r.lastBytes < 1024*1024 {
		return
	}
	if !force && r.total > 0 && percent == r.lastNotify {
		return
	}
	r.lastNotify = percent
	r.lastBytes = r.read
	r.notify(downloadProgress{
		DownloadedBytes: r.read,
		TotalBytes:      r.total,
		Progress:        percent,
	})
}

func extractZip(archivePath, destRoot string) error {
	return extractZipWithProgress(archivePath, destRoot, nil)
}

func extractZipWithProgress(archivePath, destRoot string, progress func(extractProgress)) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	totalEntries := len(reader.File)
	for index, file := range reader.File {
		targetPath := filepath.Join(destRoot, filepath.FromSlash(file.Name))
		if !pathWithinRoot(destRoot, targetPath) {
			return fmt.Errorf("zip entry escapes destination: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			in.Close()
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			in.Close()
			return err
		}
		out.Close()
		in.Close()
		if progress != nil {
			progress(extractProgress{
				ExtractedEntries: index + 1,
				TotalEntries:     totalEntries,
				Progress:         prepareProgressPercent(int64(index+1), int64(totalEntries)),
			})
		}
	}
	if progress != nil {
		progress(extractProgress{
			ExtractedEntries: totalEntries,
			TotalEntries:     totalEntries,
			Progress:         100,
		})
	}
	return nil
}

func extractTarGz(archivePath, destRoot string) error {
	return extractTarGzWithProgress(archivePath, destRoot, nil)
}

func extractTarGzWithProgress(archivePath, destRoot string, progress func(extractProgress)) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	totalEntries, err := countTarGzEntries(archivePath)
	if err != nil {
		totalEntries = 0
	}
	reader := tar.NewReader(gzr)
	extractedEntries := 0
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			if progress != nil {
				progress(extractProgress{
					ExtractedEntries: extractedEntries,
					TotalEntries:     totalEntries,
					Progress:         100,
				})
			}
			return nil
		}
		if err != nil {
			return err
		}
		targetPath := filepath.Join(destRoot, filepath.FromSlash(header.Name))
		if !pathWithinRoot(destRoot, targetPath) {
			return fmt.Errorf("tar entry escapes destination: %s", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, 0:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, reader); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
		extractedEntries++
		if progress != nil {
			progress(extractProgress{
				ExtractedEntries: extractedEntries,
				TotalEntries:     totalEntries,
				Progress:         prepareProgressPercent(int64(extractedEntries), int64(totalEntries)),
			})
		}
	}
}

func countTarGzEntries(archivePath string) (int, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return 0, err
	}
	defer gzr.Close()
	reader := tar.NewReader(gzr)
	total := 0
	for {
		_, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return total, nil
		}
		if err != nil {
			return total, err
		}
		total++
	}
}
