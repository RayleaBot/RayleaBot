package deps

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
)

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
	SystemBrowserPath    string
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
	UsedSystemBrowser  bool
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

type PrepareProgress struct {
	Kind             string `json:"kind"`
	Label            string `json:"label"`
	ResourceID       string `json:"resource_id,omitempty"`
	Version          string `json:"version,omitempty"`
	SourceLabel      string `json:"source_label,omitempty"`
	SourceURL        string `json:"source_url,omitempty"`
	ArchivePath      string `json:"archive_path,omitempty"`
	StoreRoot        string `json:"store_root,omitempty"`
	Stage            string `json:"stage"`
	Status           string `json:"status"`
	Progress         int    `json:"progress,omitempty"`
	DownloadedBytes  int64  `json:"downloaded_bytes,omitempty"`
	TotalBytes       int64  `json:"total_bytes,omitempty"`
	ExtractedEntries int    `json:"extracted_entries,omitempty"`
	TotalEntries     int    `json:"total_entries,omitempty"`
	Summary          string `json:"summary,omitempty"`
	Error            string `json:"error,omitempty"`
}

type PrepareProgressReporter func(PrepareProgress)

type PrepareOptions struct {
	Progress PrepareProgressReporter
}

type downloadProgress struct {
	DownloadedBytes int64
	TotalBytes      int64
	Progress        int
}

type extractProgress struct {
	ExtractedEntries int
	TotalEntries     int
	Progress         int
}

// Runtime exposes dependency operations that may prepare or resolve runtime entrypoints.
type Runtime struct {
	manager *Manager
}

func NewRuntime(repoRoot string) *Runtime {
	return &Runtime{manager: NewManager(repoRoot)}
}

func (r *Runtime) ResolveEntrypoint(ctx context.Context, kind, name string) (string, error) {
	if r == nil || r.manager == nil {
		return "", errManagerRequired()
	}
	return r.manager.ResolveEntrypoint(ctx, kind, name)
}

func (r *Runtime) ResolvePreparedEntrypoint(kind, name string) (string, error) {
	if r == nil || r.manager == nil {
		return "", errManagerRequired()
	}
	return r.manager.ResolvePreparedEntrypoint(kind, name)
}

func (r *Runtime) PrepareWithReport(ctx context.Context, kind string) (*PrepareReport, error) {
	if r == nil || r.manager == nil {
		return nil, errManagerRequired()
	}
	return r.manager.PrepareWithReport(ctx, kind)
}

func (r *Runtime) PrepareWithReportOptions(ctx context.Context, kind string, options PrepareOptions) (*PrepareReport, error) {
	if r == nil || r.manager == nil {
		return nil, errManagerRequired()
	}
	return r.manager.PrepareWithReportOptions(ctx, kind, options)
}

// Diagnostics exposes read-only dependency status checks.
type Diagnostics struct {
	manager *Manager
}

func NewDiagnostics(repoRoot string) *Diagnostics {
	return &Diagnostics{manager: NewManager(repoRoot)}
}

func (d *Diagnostics) InspectRuntime(kind string) (*BootstrapInspection, error) {
	if d == nil || d.manager == nil {
		return nil, errManagerRequired()
	}
	return d.manager.Inspect(kind)
}

func errManagerRequired() error {
	return errors.New("deps manager is required")
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

func pathWithinRoot(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
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

func ManagedResourceLabel(kind string) string {
	return managedResourceLabel(kind)
}

func ManagedResourceText(kind, suffix string) string {
	return managedResourceText(kind, suffix)
}

func BootstrapRemediation(kind, archivePath, storeRoot string) string {
	return bootstrapRemediation(kind, archivePath, storeRoot)
}

func BootstrapSummary(kind string, inspection *BootstrapInspection) string {
	switch {
	case inspection == nil:
		return managedResourceText(kind, "清单不可用。")
	case !inspection.MetadataComplete:
		return managedResourceText(kind, "元数据不完整。")
	case inspection.PreparedStorePresent:
		return managedResourceText(kind, "已准备完成。")
	case inspection.CachedArchivePresent:
		if kind == "python-runtime" || kind == "nodejs-runtime" {
			return managedResourceText(kind, "已下载，启动时会解压。")
		}
		return managedResourceText(kind, "已下载，未解压。")
	default:
		if kind == "python-runtime" || kind == "nodejs-runtime" {
			return managedResourceText(kind, "已纳入启动流程。")
		}
		return managedResourceText(kind, "未准备。")
	}
}

func bootstrapMessage(kind, stage string) string {
	switch stage {
	case "manifest":
		return managedResourceText(kind, "清单不可用")
	case "lock":
		return managedResourceText(kind, "准备锁等待超时")
	case "download":
		return managedResourceText(kind, "安装包下载失败")
	case "verify":
		return managedResourceText(kind, "安装包校验失败")
	case "extract":
		return managedResourceText(kind, "安装包解压失败")
	case "entrypoint":
		return managedResourceText(kind, "入口文件缺失")
	default:
		return managedResourceText(kind, "准备失败")
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
		return "启动运行环境任务准备图片渲染 Chromium，或在配置中设置 render.browser_path。" + locationText
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
		return "图片渲染 Chromium"
	case "python-runtime":
		return "Python 运行环境"
	case "nodejs-runtime":
		return "Node.js / npm 环境"
	default:
		return "运行环境"
	}
}

func managedResourceText(kind, suffix string) string {
	label := managedResourceLabel(kind)
	if kind == "chromium" && strings.TrimSpace(suffix) != "" {
		return label + " " + suffix
	}
	return label + suffix
}

func (p PrepareProgress) withResource(resource *Resource, archivePath, storeRoot string) PrepareProgress {
	if resource == nil {
		p.Label = managedResourceLabel(p.Kind)
		p.ArchivePath = strings.TrimSpace(archivePath)
		p.StoreRoot = strings.TrimSpace(storeRoot)
		return p
	}
	p.Kind = resource.Kind
	p.Label = managedResourceLabel(resource.Kind)
	p.ResourceID = resource.ID
	p.Version = resource.Version
	p.ArchivePath = strings.TrimSpace(archivePath)
	p.StoreRoot = strings.TrimSpace(storeRoot)
	return p
}

func emitPrepareProgress(reporter PrepareProgressReporter, event PrepareProgress) {
	if reporter == nil {
		return
	}
	if event.Label == "" {
		event.Label = managedResourceLabel(event.Kind)
	}
	reporter(event)
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
