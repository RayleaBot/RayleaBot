package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	renderbrowser "github.com/RayleaBot/RayleaBot/server/internal/render/browser"
	rendercatalog "github.com/RayleaBot/RayleaBot/server/internal/render/catalog"
	renderworker "github.com/RayleaBot/RayleaBot/server/internal/render/engine"
	renderrepo "github.com/RayleaBot/RayleaBot/server/internal/render/repository"
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

var revisionCounter uint64

type Options struct {
	RepoRoot           string
	OutputRoot         string
	Store              *storage.Store
	Runner             renderbrowser.Runner
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
	Template string         `json:"template"`
	Theme    string         `json:"theme,omitempty"`
	Output   string         `json:"output,omitempty"`
	Data     map[string]any `json:"data"`
	Plugin   *PluginContext `json:"-"`
}

type PluginContext struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

type PreviewHTML struct {
	TemplateID string
	RevisionID string
	Width      int
	Height     int
	HTML       string
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
	worker         *renderworker.Worker
	logger         *slog.Logger
	templateRepo   *renderrepo.SQLiteTemplateRepository
	templateSyncMu sync.Mutex
	templateRoots  *rendercatalog.Roots

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

	templateRepo, err := renderrepo.NewSQLiteTemplateRepository(options.Store)
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
		if managedBrowser, err := deps.NewManager(repoRoot).ResolvePreparedEntrypoint("chromium", "browser"); err == nil {
			browserPath = managedBrowser
		}
	}

	runner := options.Runner
	if runner == nil {
		runner = renderbrowser.NewChromiumRunner(renderbrowser.ChromiumOptions{
			BrowserPath: browserPath,
			BrowserArgs: options.BrowserArgs,
		})
	}

	service := &Service{
		repoRoot:           repoRoot,
		templatesRoot:      templatesRoot,
		outputRoot:         outputRoot,
		browserPath:        browserPath,
		browserArgs:        append([]string(nil), options.BrowserArgs...),
		logger:             options.Logger,
		config:             newRuntimeConfig(maxRenderDataBytes, footerTemplate, defaultOutput, deviceScalePercent),
		templateRepo:       templateRepo,
		templateRoots:      rendercatalog.NewRoots(templatesRoot),
		artifactStore:      newArtifactStore(outputRoot),
	}
	service.worker = renderworker.New(renderworker.Config{
		Runner:           runner,
		WorkerCount:      workerCount,
		QueueMaxLength:   queueMaxLength,
		QueueWaitTimeout: queueWaitTimeout,
		RenderTimeout:    renderTimeout,
		OnQueueDepth:     service.publishQueueDepth,
	})

	if err := service.syncTemplatesFromFiles(context.Background()); err != nil {
		return nil, err
	}
	if err := service.artifactStore.load(); err != nil {
		return nil, err
	}

	return service, nil
}
