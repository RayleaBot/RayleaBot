package render

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
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
	renderCacheVersion      = "render-cache-v3-template-sources"
)

var artifactIDPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)
var pluginTemplateLocalIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
var revisionCounter uint64

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

	templateRepo, err := newSQLiteTemplateRepository(options.Store)
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
		runner = NewChromiumRunner(ChromiumOptions{
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
		runner:             runner,
		workerSem:          make(chan struct{}, workerCount),
		workerCount:        workerCount,
		logger:             options.Logger,
		queueMaxLength:     queueMaxLength,
		queueWaitTimeout:   queueWaitTimeout,
		renderTimeout:      renderTimeout,
		maxRenderDataBytes: maxRenderDataBytes,
		footerTemplate:     footerTemplate,
		defaultOutput:      defaultOutput,
		deviceScalePercent: deviceScalePercent,
		templateRepo:       templateRepo,
		templateRoots:      map[string]templateRoot{},
		cache:              map[string]Result{},
		artifacts:          map[string]Artifact{},
		previewHTMLCache:   map[string]PreviewHTML{},
	}

	if err := service.syncTemplatesFromFiles(context.Background()); err != nil {
		return nil, err
	}
	if err := service.loadArtifacts(); err != nil {
		return nil, err
	}

	return service, nil
}
