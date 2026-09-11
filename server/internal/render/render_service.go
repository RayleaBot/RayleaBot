package render

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/health"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

const (
	defaultWorkerCount      = 1
	defaultQueueMaxLength   = 32
	defaultQueueWaitTimeout = 15 * time.Second
	defaultRenderTimeout    = 20 * time.Second
	defaultRenderDataLimit  = 1 << 20
	defaultRenderFooter     = "Created By RayleaBot {{rayleabot_version}} & Plugin {{plugin_name}} {{plugin_version}}"
	defaultRenderOutput     = "png"
	defaultDeviceScalePct   = 100
	developmentVersion      = "开发版本"
	systemTemplatePlugin    = "系统模板"
)

type TemplateSource struct {
	ManifestJSON    map[string]any `json:"manifest_json"`
	HTML            string         `json:"html"`
	Stylesheet      string         `json:"stylesheet"`
	InputSchemaJSON map[string]any `json:"input_schema_json"`
}

type TemplateFiles struct {
	Manifest    string  `json:"manifest"`
	HTML        string  `json:"html"`
	Stylesheet  string  `json:"stylesheet"`
	InputSchema *string `json:"input_schema"`
}

type TemplateSourceInfo struct {
	Type     string `json:"type"`
	PluginID string `json:"plugin_id,omitempty"`
	LocalID  string `json:"local_id,omitempty"`
}

type TemplateSummary struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	Version        string `json:"version"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	HasInputSchema bool   `json:"has_input_schema"`
	SourceDigest   string `json:"source_digest"`
	UpdatedAt      string `json:"updated_at"`
	Source         TemplateSourceInfo
}

type TemplateDetail struct {
	TemplateSummary
	Files TemplateFiles `json:"files"`
}

type TemplateDetailSnapshot struct {
	Detail      TemplateDetail
	Source      TemplateSource
	PreviewData map[string]any
}

type Options struct {
	RepoRoot           string
	OutputRoot         string
	Store              *storage.Store
	Runner             Runner
	WorkerCount        int
	BrowserArgs        []string
	BrowserPath        string
	QueueMaxLength     int
	QueueWaitTimeout   time.Duration
	RenderTimeout      time.Duration
	MaxRenderDataBytes int
	FooterTemplate     string
	DefaultOutput      string
	DeviceScalePercent int
	Logger             *slog.Logger
	InspectRuntime     func(string) (*deps.BootstrapInspection, error)
}

type RuntimeConfig struct {
	QueueMaxLength     int
	QueueWaitTimeout   time.Duration
	RenderTimeout      time.Duration
	FooterTemplate     string
	DefaultOutput      string
	DeviceScalePercent int
}

type Request struct {
	Template  string           `json:"template"`
	Theme     string           `json:"theme,omitempty"`
	Output    string           `json:"output,omitempty"`
	Data      map[string]any   `json:"data"`
	Resources []RenderResource `json:"-"`
	Plugin    *PluginContext   `json:"-"`
}

type RenderResource struct {
	ID     string
	Path   string
	MIME   string
	SHA256 string
	Size   int64
}

type PluginContext struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

type PreviewHTML struct {
	TemplateID   string
	SourceDigest string
	Width        int
	Height       int
	HTML         string
}

type TemplateAsset struct {
	Path string
}

type Service struct {
	repoRoot       string
	templatesRoot  string
	outputRoot     string
	browserPath    string
	browserArgs    []string
	worker         *Worker
	logger         *slog.Logger
	templateRepo   *templateRepository
	templateSyncMu sync.Mutex
	templateRoots  *Roots
	inspectRuntime func(string) (*deps.BootstrapInspection, error)

	mu sync.RWMutex

	config        *runtimeConfig
	artifactStore *artifactStore

	metricsMu sync.RWMutex
	metrics   MetricsObserver
}

// MetricsObserver routes render service outcomes into the Prometheus registry.
type MetricsObserver interface {
	SetRenderQueueDepth(depth int)
	ObserveRenderDuration(outcome string, duration time.Duration)
}

func (s *Service) SetMetricsObserver(observer MetricsObserver) {
	s.metricsMu.Lock()
	s.metrics = observer
	s.metricsMu.Unlock()
}

func (s *Service) currentMetrics() MetricsObserver {
	s.metricsMu.RLock()
	defer s.metricsMu.RUnlock()
	return s.metrics
}

func (s *Service) recordRenderMetric(outcome string, duration time.Duration) {
	observer := s.currentMetrics()
	if observer == nil {
		return
	}
	observer.ObserveRenderDuration(outcome, duration)
}

func renderOutcome(result Result, err error) string {
	if err != nil {
		var renderErr *Error
		if errors.As(err, &renderErr) {
			switch renderErr.Code {
			case errorcodes.PlatformRenderQueueFull:
				return "queue_full"
			case errorcodes.PlatformRenderTimeout:
				return "timeout"
			}
		}
		return "failed"
	}
	if result.FromCache {
		return "cache_hit"
	}
	return "succeeded"
}

func (s *Service) publishQueueDepth(depth int) {
	observer := s.currentMetrics()
	if observer == nil {
		return
	}
	go observer.SetRenderQueueDepth(depth)
}

func NewService(options Options) (*Service, error) {
	repoRoot, err := filepath.Abs(options.RepoRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve render repo root %s: %w", options.RepoRoot, err)
	}
	outputRoot, err := filepath.Abs(options.OutputRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve render output root %s: %w", options.OutputRoot, err)
	}
	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create render output root %s: %w", outputRoot, err)
	}

	templateRepo, err := newTemplateRepository(options.Store)
	if err != nil {
		return nil, fmt.Errorf("create render template repository: %w", err)
	}
	templatesRoot := filepath.Join(repoRoot, "templates")

	workerCount := options.WorkerCount
	if workerCount <= 0 {
		workerCount = defaultWorkerCount
	}
	queueMaxLength := options.QueueMaxLength
	if queueMaxLength <= 0 {
		queueMaxLength = defaultQueueMaxLength
	}
	queueWaitTimeout := options.QueueWaitTimeout
	if queueWaitTimeout <= 0 {
		queueWaitTimeout = defaultQueueWaitTimeout
	}
	renderTimeout := options.RenderTimeout
	if renderTimeout <= 0 {
		renderTimeout = defaultRenderTimeout
	}
	maxRenderDataBytes := options.MaxRenderDataBytes
	if maxRenderDataBytes <= 0 {
		maxRenderDataBytes = defaultRenderDataLimit
	}
	footerTemplate := strings.TrimSpace(options.FooterTemplate)
	if footerTemplate == "" {
		footerTemplate = defaultRenderFooter
	}
	defaultOutput := normalizeDefaultOutput(options.DefaultOutput)
	deviceScalePercent := normalizeDeviceScalePercent(options.DeviceScalePercent)

	browserPath := strings.TrimSpace(options.BrowserPath)
	if browserPath == "" {
		if managedBrowser, err := deps.NewRuntime(repoRoot).ResolvePreparedEntrypoint("chromium", "browser"); err == nil {
			browserPath = managedBrowser
		}
	}

	runner := options.Runner
	if runner == nil {
		runner = NewChromiumRunner(ChromiumOptions{
			BrowserPath: browserPath,
			BrowserArgs: options.BrowserArgs,
		})
	}

	service := &Service{
		repoRoot:       repoRoot,
		templatesRoot:  templatesRoot,
		outputRoot:     outputRoot,
		browserPath:    browserPath,
		browserArgs:    append([]string(nil), options.BrowserArgs...),
		logger:         options.Logger,
		inspectRuntime: options.InspectRuntime,
		config:         newRuntimeConfig(maxRenderDataBytes, footerTemplate, defaultOutput, deviceScalePercent),
		templateRepo:   templateRepo,
		templateRoots:  NewRoots(templatesRoot),
		artifactStore:  newArtifactStore(outputRoot),
	}
	service.worker = NewWorker(WorkerConfig{
		Runner:           runner,
		WorkerCount:      workerCount,
		QueueMaxLength:   queueMaxLength,
		QueueWaitTimeout: queueWaitTimeout,
		RenderTimeout:    renderTimeout,
		OnQueueDepth:     service.publishQueueDepth,
	})

	if err := service.syncTemplatesFromFiles(context.Background()); err != nil {
		return nil, errors.Join(err, service.Close())
	}
	if err := service.artifactStore.load(); err != nil {
		return nil, errors.Join(err, service.Close())
	}

	return service, nil
}

func (s *Service) Close() error {
	return s.worker.Close()
}

func (s *Service) RefreshBrowserPath(browserPath string) {
	trimmed := strings.TrimSpace(browserPath)
	s.mu.Lock()
	s.browserPath = trimmed
	browserArgs := append([]string(nil), s.browserArgs...)
	s.mu.Unlock()

	s.worker.RefreshChromiumRunner(trimmed, browserArgs)
}

func (s *Service) currentRunner() Runner {
	return s.worker.CurrentRunner()
}

func (s *Service) BrowserLaunchConfig() (string, []string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.browserPath, append([]string(nil), s.browserArgs...)
}

// runtimeConfig holds the hot-reloadable render tuning values behind its own
// lock, separate from the render service's browser-runner state.
type runtimeConfig struct {
	mu                 sync.RWMutex
	maxRenderDataBytes int
	footerTemplate     string
	defaultOutput      string
	deviceScalePercent int
}

type renderSettings struct {
	maxRenderDataBytes int
	footerTemplate     string
	defaultOutput      string
	deviceScalePercent int
}

func (c *runtimeConfig) snapshot() renderSettings {
	c.mu.RLock()
	defer c.mu.RUnlock()
	footer := c.footerTemplate
	if strings.TrimSpace(footer) == "" {
		footer = defaultRenderFooter
	}
	return renderSettings{
		maxRenderDataBytes: c.maxRenderDataBytes, footerTemplate: footer,
		defaultOutput:      normalizeDefaultOutput(c.defaultOutput),
		deviceScalePercent: normalizeDeviceScalePercent(c.deviceScalePercent),
	}
}

func newRuntimeConfig(maxRenderDataBytes int, footerTemplate, defaultOutput string, deviceScalePercent int) *runtimeConfig {
	return &runtimeConfig{
		maxRenderDataBytes: maxRenderDataBytes,
		footerTemplate:     footerTemplate,
		defaultOutput:      defaultOutput,
		deviceScalePercent: deviceScalePercent,
	}
}

func (c *runtimeConfig) update(config RuntimeConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.footerTemplate = config.FooterTemplate
	if strings.TrimSpace(config.DefaultOutput) != "" {
		c.defaultOutput = normalizeDefaultOutput(config.DefaultOutput)
	}
	if config.DeviceScalePercent > 0 {
		c.deviceScalePercent = normalizeDeviceScalePercent(config.DeviceScalePercent)
	}
}

func (s *Service) ApplyConfig(cfg config.Config) {
	s.UpdateRuntimeConfig(RuntimeConfig{
		QueueMaxLength:     cfg.Render.QueueMaxLength,
		QueueWaitTimeout:   time.Duration(cfg.Render.QueueWaitTimeoutSeconds) * time.Second,
		RenderTimeout:      time.Duration(cfg.Render.TimeoutSeconds) * time.Second,
		FooterTemplate:     cfg.Render.FooterTemplate,
		DefaultOutput:      cfg.Render.DefaultOutput,
		DeviceScalePercent: cfg.Render.DeviceScalePercent,
	})
}

func (s *Service) UpdateRuntimeConfig(config RuntimeConfig) {
	s.worker.UpdateLimits(WorkerLimits{
		QueueMaxLength:   config.QueueMaxLength,
		QueueWaitTimeout: config.QueueWaitTimeout,
		RenderTimeout:    config.RenderTimeout,
	})
	s.config.update(config)
}

func normalizeDefaultOutput(output string) string {
	switch strings.TrimSpace(strings.ToLower(output)) {
	case "jpeg":
		return "jpeg"
	default:
		return defaultRenderOutput
	}
}

func normalizeDeviceScalePercent(percent int) int {
	if percent < 50 || percent > 500 {
		return defaultDeviceScalePct
	}
	return percent
}

func deviceScaleFactorFromPercent(percent int) float64 {
	normalized := normalizeDeviceScalePercent(percent)
	return float64(normalized) / 100
}

func (s *Service) Diagnostics() []health.DiagnosticIssue {
	issues := make([]health.DiagnosticIssue, 0, 4)

	info, err := os.Stat(s.templatesRoot)
	switch {
	case os.IsNotExist(err):
		issues = append(issues, health.DiagnosticIssue{
			Code:        errorcodes.PlatformResourceMissing,
			Severity:    "warning",
			Summary:     "模板资源目录缺失",
			Remediation: "请恢复仓库中的 templates 目录。",
		})
	case err != nil:
		issues = append(issues, health.DiagnosticIssue{
			Code:        errorcodes.PlatformResourceMissing,
			Severity:    "warning",
			Summary:     "模板资源目录不可读",
			Remediation: "请确认 templates 目录存在且当前进程有读取权限。",
		})
	case !info.IsDir():
		issues = append(issues, health.DiagnosticIssue{
			Code:        errorcodes.PlatformResourceMissing,
			Severity:    "warning",
			Summary:     "模板资源目录结构无效",
			Remediation: "请恢复仓库中的 templates 目录结构。",
		})
	default:
		Seeds, err := DiscoverSeeds(s.repoRoot, s.templatesRoot, s.logger)
		if err != nil {
			issues = append(issues, health.DiagnosticIssue{
				Code:        errorcodes.PlatformResourceMissing,
				Severity:    "warning",
				Summary:     "模板资源目录不可读",
				Remediation: "请确认 templates 目录存在且当前进程有读取权限。",
			})
			break
		}
		required := []string{"help.menu", "status.panel"}
		for _, templateID := range required {
			if _, ok := Seeds[templateID]; ok {
				continue
			}
			issues = append(issues, health.DiagnosticIssue{
				Code:        errorcodes.PlatformResourceMissing,
				Severity:    "warning",
				Summary:     fmt.Sprintf("渲染模板 %s 缺失", templateID),
				Remediation: "请恢复仓库中的正式模板资源。",
			})
		}
	}

	browserPath, _ := s.BrowserLaunchConfig()
	if strings.TrimSpace(browserPath) != "" {
		return issues
	}

	inspect := s.inspectRuntime
	if inspect == nil {
		inspect = deps.NewDiagnostics(s.repoRoot).InspectRuntime
	}
	inspection, err := inspect("chromium")
	if err != nil {
		var bootstrapErr *deps.BootstrapError
		if errors.As(err, &bootstrapErr) {
			issues = append(issues, health.DiagnosticIssue{
				RuntimeResources: []string{"chromium"},
				Code:             errorcodes.PlatformResourceMissing,
				Severity:         "warning",
				Summary:          bootstrapErr.Message,
				Remediation:      bootstrapErr.Remediation,
			})
			return issues
		}
		issues = append(issues, health.DiagnosticIssue{
			RuntimeResources: []string{"chromium"},
			Code:             errorcodes.PlatformResourceMissing,
			Severity:         "warning",
			Summary:          "图片渲染 Chromium 资源清单不可用。",
			Remediation:      "请恢复 .deps/manifest.json，或在配置中显式设置 render.browser_path。",
		})
		return issues
	}
	if !inspection.MetadataComplete {
		issues = append(issues, health.DiagnosticIssue{
			RuntimeResources: []string{"chromium"},
			Code:             errorcodes.PlatformResourceMissing,
			Severity:         "warning",
			Summary:          deps.BootstrapSummary("chromium", inspection),
			Remediation:      "请恢复当前平台图片渲染 Chromium 资源的 archive_format、entrypoints、来源列表与 sha256，或在配置中显式设置 render.browser_path。",
		})
		return issues
	}
	if inspection.PreparedStorePresent {
		return issues
	}
	if inspection.CachedArchivePresent {
		issues = append(issues, health.DiagnosticIssue{
			RuntimeResources: []string{"chromium"},
			Code:             errorcodes.PlatformResourceMissing,
			Severity:         "warning",
			Summary:          "图片渲染 Chromium 已下载，但未解压。",
			Remediation:      deps.BootstrapRemediation("chromium", inspection.ArchivePath, inspection.StoreRoot),
		})
		return issues
	}
	issues = append(issues, health.DiagnosticIssue{
		RuntimeResources: []string{"chromium"},
		Code:             errorcodes.PlatformResourceMissing,
		Severity:         "warning",
		Summary:          "图片渲染 Chromium 未准备。",
		Remediation:      deps.BootstrapRemediation("chromium", inspection.ArchivePath, inspection.StoreRoot),
	})
	return issues
}
