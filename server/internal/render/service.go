package render

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

const (
	defaultWorkerCount      = 1
	defaultQueueMaxLength   = 32
	defaultQueueWaitTimeout = 15 * time.Second
	defaultRenderTimeout    = 20 * time.Second
	defaultRenderDataLimit  = 1 << 20
	defaultRenderFooter     = "Created By RayleaBot {{rayleabot_version}} & Plugin {{plugin_name}} {{plugin_version}}"
	developmentVersion      = "开发版本"
	systemTemplatePlugin    = "系统模板"
	renderCacheVersion      = "render-cache-v3-template-sources"
)

var artifactIDPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)
var pluginTemplateLocalIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
var revisionCounter uint64

type Runner interface {
	Render(ctx context.Context, doc Document) ([]byte, error)
}

type closeableRunner interface {
	Close() error
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
	Logger             *slog.Logger
}

type RuntimeConfig struct {
	QueueMaxLength   int
	QueueWaitTimeout time.Duration
	RenderTimeout    time.Duration
	FooterTemplate   string
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

type Document struct {
	Template   string
	Theme      string
	Output     string
	BaseURL    string
	Width      int
	Height     int
	AutoHeight bool
	HTML       string
}

type Result struct {
	ArtifactID string
	ImagePath  string
	MIME       string
	CacheKey   string
	Template   string
	Theme      string
	FromCache  bool
}

type Artifact struct {
	ArtifactID string
	MIME       string
	Path       string
}

type Error struct {
	Code    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type artifactRecord struct {
	ArtifactID string `json:"artifact_id"`
	CacheKey   string `json:"cache_key"`
	Template   string `json:"template"`
	Theme      string `json:"theme"`
	Output     string `json:"output"`
	MIME       string `json:"mime"`
	Filename   string `json:"filename"`
}

type Service struct {
	repoRoot       string
	templatesRoot  string
	outputRoot     string
	browserPath    string
	browserArgs    []string
	runner         Runner
	workerSem      chan struct{}
	workerCount    int
	logger         *slog.Logger
	templateRepo   *sqliteTemplateRepository
	templateSyncMu sync.Mutex
	templateRoots  map[string]string

	mu                 sync.RWMutex
	queueMaxLength     int
	queueWaitTimeout   time.Duration
	renderTimeout      time.Duration
	maxRenderDataBytes int
	footerTemplate     string
	activeRequests     int
	cache              map[string]Result
	artifacts          map[string]Artifact
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
		templateRepo:       templateRepo,
		templateRoots:      map[string]string{},
		cache:              map[string]Result{},
		artifacts:          map[string]Artifact{},
	}

	if err := service.syncTemplatesFromFiles(context.Background()); err != nil {
		return nil, err
	}
	if err := service.loadArtifacts(); err != nil {
		return nil, err
	}

	return service, nil
}

func (s *Service) Close() error {
	if s == nil {
		return nil
	}
	releaseWorkers := s.acquireAllWorkerSlots()
	defer releaseWorkers()

	s.mu.Lock()
	runner := s.runner
	s.runner = nil
	s.mu.Unlock()
	return closeRenderRunner(runner)
}

func (s *Service) RefreshBrowserPath(browserPath string) {
	if s == nil {
		return
	}

	trimmed := strings.TrimSpace(browserPath)
	s.mu.Lock()
	s.browserPath = trimmed
	oldRunner := s.runner
	browserArgs := append([]string(nil), s.browserArgs...)
	_, replaceDefaultRunner := oldRunner.(*chromiumRunner)
	s.mu.Unlock()

	if !replaceDefaultRunner {
		return
	}

	releaseWorkers := s.acquireAllWorkerSlots()
	defer releaseWorkers()

	s.mu.Lock()
	if s.runner != oldRunner {
		s.mu.Unlock()
		return
	}
	s.runner = NewChromiumRunner(ChromiumOptions{
		BrowserPath: trimmed,
		BrowserArgs: browserArgs,
	})
	s.mu.Unlock()
	_ = closeRenderRunner(oldRunner)
}

func (s *Service) UpdateRuntimeConfig(config RuntimeConfig) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if config.QueueMaxLength > 0 {
		s.queueMaxLength = config.QueueMaxLength
	}
	if config.QueueWaitTimeout > 0 {
		s.queueWaitTimeout = config.QueueWaitTimeout
	}
	if config.RenderTimeout > 0 {
		s.renderTimeout = config.RenderTimeout
	}
	if strings.TrimSpace(config.FooterTemplate) != "" {
		s.footerTemplate = config.FooterTemplate
	}
}

func (s *Service) Render(ctx context.Context, request Request) (Result, error) {
	if s == nil {
		return Result{}, &Error{Code: "platform.resource_missing", Message: "render service is not available"}
	}

	normalized, payloadBytes, err := s.normalizeRequest(request)
	if err != nil {
		return Result{}, err
	}

	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return Result{}, err
	}

	compiled, cacheVersion, cacheDigest, err := s.resolveCompiledTemplate(ctx, normalized)
	if err != nil {
		return Result{}, err
	}
	templateDir := s.templateDirFor(normalized.Template)
	resourceDigest := templateResourceDigest(templateDir)
	cacheKey := buildCacheKey(normalized, cacheVersion, cacheDigest, resourceDigest, payloadBytes)
	if cached, ok := s.cachedResult(cacheKey); ok {
		cached.FromCache = true
		return cached, nil
	}

	if err := s.reserveSlot(); err != nil {
		return Result{}, err
	}
	defer s.releaseSlot()

	queueCtx := ctx
	if timeout := s.currentQueueWaitTimeout(); timeout > 0 {
		var cancel context.CancelFunc
		queueCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	select {
	case s.workerSem <- struct{}{}:
	case <-queueCtx.Done():
		return Result{}, &Error{
			Code:    "platform.render_timeout",
			Message: "render queue wait timed out",
			Err:     queueCtx.Err(),
		}
	}
	defer func() {
		<-s.workerSem
	}()

	if cached, ok := s.cachedResult(cacheKey); ok {
		cached.FromCache = true
		return cached, nil
	}

	html, err := compiled.renderHTML(normalized.Theme, normalized.Data)
	if err != nil {
		return Result{}, wrapRenderError(err, "render template execution failed")
	}

	renderCtx := ctx
	if timeout := s.currentRenderTimeout(); timeout > 0 {
		var cancel context.CancelFunc
		renderCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	runner := s.currentRunner()
	if runner == nil {
		return Result{}, &Error{Code: "platform.resource_missing", Message: "render runner is not available"}
	}
	content, err := runner.Render(renderCtx, Document{
		Template:   normalized.Template,
		Theme:      normalized.Theme,
		Output:     normalized.Output,
		BaseURL:    templateBaseURL(templateDir),
		Width:      compiled.bundle.manifest.Width,
		Height:     compiled.bundle.manifest.Height,
		AutoHeight: true,
		HTML:       html,
	})
	if err != nil {
		if errors.Is(renderCtx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			return Result{}, &Error{
				Code:    "platform.render_timeout",
				Message: "render execution timed out",
				Err:     err,
			}
		}
		return Result{}, wrapRenderError(err, "render execution failed")
	}

	result, err := s.persistArtifact(normalized, cacheKey, content)
	if err != nil {
		return Result{}, err
	}

	s.mu.Lock()
	s.cache[cacheKey] = result
	s.mu.Unlock()

	return result, nil
}

func (s *Service) currentRunner() Runner {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.runner
}

func (s *Service) acquireAllWorkerSlots() func() {
	if s == nil || s.workerSem == nil {
		return func() {}
	}
	count := cap(s.workerSem)
	for i := 0; i < count; i++ {
		s.workerSem <- struct{}{}
	}
	return func() {
		for i := 0; i < count; i++ {
			<-s.workerSem
		}
	}
}

func closeRenderRunner(runner Runner) error {
	closeable, ok := runner.(closeableRunner)
	if !ok || closeable == nil {
		return nil
	}
	return closeable.Close()
}

func (s *Service) ListTemplates(ctx context.Context) ([]TemplateSummary, error) {
	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return nil, err
	}

	items, err := s.templateRepo.ListTemplateSummaries(ctx)
	if err != nil {
		return nil, fmt.Errorf("list render templates: %w", err)
	}
	return items, nil
}

func (s *Service) GetTemplate(ctx context.Context, templateID string) (TemplateDetail, error) {
	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return TemplateDetail{}, err
	}

	detail, err := s.templateRepo.GetTemplateDetail(ctx, strings.TrimSpace(templateID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TemplateDetail{}, &Error{
				Code:    "platform.template_not_found",
				Message: "render template was not found",
			}
		}
		return TemplateDetail{}, fmt.Errorf("get render template %s: %w", templateID, err)
	}
	return detail, nil
}

func (s *Service) GetTemplateSource(ctx context.Context, templateID string) (string, TemplateSource, error) {
	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return "", TemplateSource{}, err
	}

	revisionID, source, err := s.templateRepo.GetCurrentSource(ctx, strings.TrimSpace(templateID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", TemplateSource{}, &Error{
				Code:    "platform.template_not_found",
				Message: "render template was not found",
			}
		}
		return "", TemplateSource{}, fmt.Errorf("get render template source %s: %w", templateID, err)
	}
	return revisionID, source, nil
}

func (s *Service) GetTemplatePreviewData(ctx context.Context, templateID string) (map[string]any, error) {
	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return nil, err
	}
	templateID = strings.TrimSpace(templateID)
	if _, err := s.GetTemplate(ctx, templateID); err != nil {
		return nil, err
	}

	templateDir := s.templateDirFor(templateID)
	previewPath := filepath.Join(templateDir, defaultTemplatePreviewData)
	content, err := os.ReadFile(previewPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read render template preview data %s: %w", previewPath, err)
	}

	var previewData map[string]any
	if err := json.Unmarshal(content, &previewData); err != nil {
		return nil, &Error{
			Code:    "platform.template_source_invalid",
			Message: "render template preview data is invalid",
			Err:     err,
		}
	}
	return previewData, nil
}

func (s *Service) ResolvePluginTemplate(ctx context.Context, pluginID, requested string) (string, error) {
	if s == nil {
		return "", &Error{Code: "platform.resource_missing", Message: "render service is not available"}
	}
	pluginID = strings.TrimSpace(pluginID)
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return "", &Error{Code: "platform.invalid_request", Message: "render template is required"}
	}
	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return "", err
	}

	if strings.HasPrefix(requested, "plugin.") {
		ownerPluginID, _, ok := parseFormalPluginTemplateID(requested)
		if !ok || pluginID == "" || ownerPluginID != pluginID {
			return "", &Error{
				Code:    "permission.scope_violation",
				Message: "plugin render template belongs to another plugin",
			}
		}
		detail, err := s.templateRepo.GetTemplateDetail(ctx, requested)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return requested, nil
			}
			return "", fmt.Errorf("get plugin render template %s: %w", requested, err)
		}
		if detail.Source.Type == "plugin" && detail.Source.PluginID != pluginID {
			return "", &Error{
				Code:    "permission.scope_violation",
				Message: "plugin render template belongs to another plugin",
			}
		}
		return requested, nil
	}

	formalID := formalPluginTemplateID(pluginID, requested)
	if detail, err := s.templateRepo.GetTemplateDetail(ctx, formalID); err == nil {
		if detail.Source.Type == "plugin" && detail.Source.PluginID == pluginID && detail.Source.LocalID == requested {
			return formalID, nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("get plugin render template %s: %w", formalID, err)
	}

	return requested, nil
}

func (s *Service) ValidateTemplate(ctx context.Context, templateID string, source *TemplateSource) (TemplateValidationResult, error) {
	templateID = strings.TrimSpace(templateID)
	if templateID == "" {
		return TemplateValidationResult{}, &Error{Code: "platform.template_not_found", Message: "render template was not found"}
	}

	if exists, err := s.templateRepo.templateExists(ctx, templateID); err != nil {
		return TemplateValidationResult{}, fmt.Errorf("query render template %s: %w", templateID, err)
	} else if !exists {
		return TemplateValidationResult{}, &Error{
			Code:    "platform.template_not_found",
			Message: "render template was not found",
		}
	}

	var sourceValue TemplateSource
	if source == nil {
		_, currentSource, err := s.templateRepo.GetCurrentSource(ctx, templateID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return TemplateValidationResult{}, &Error{
					Code:    "platform.template_not_found",
					Message: "render template was not found",
				}
			}
			return TemplateValidationResult{}, fmt.Errorf("get render template source %s: %w", templateID, err)
		}
		sourceValue = currentSource
	} else {
		sourceValue = *source
	}

	bundle, err := buildTemplateSourceBundle(templateID, sourceValue)
	if err != nil {
		_ = s.templateRepo.UpdateValidationStatus(ctx, templateID, newValidationStatus(false, 1))
		return TemplateValidationResult{}, err
	}

	_, issues, err := compileTemplateBundle(bundle)
	if err != nil {
		return TemplateValidationResult{}, fmt.Errorf("validate render template %s: %w", templateID, err)
	}

	status := newValidationStatus(len(issues) == 0, len(issues))
	if err := s.templateRepo.UpdateValidationStatus(ctx, templateID, status); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return TemplateValidationResult{}, fmt.Errorf("update render template validation %s: %w", templateID, err)
	}

	return TemplateValidationResult{
		Valid:              len(issues) == 0,
		Issues:             issuesOrEmpty(issues),
		NormalizedManifest: bundle.normalizedManifest,
	}, nil
}

func (s *Service) ListTemplateVersions(ctx context.Context, templateID string) ([]TemplateVersion, error) {
	items, err := s.templateRepo.ListTemplateVersions(ctx, strings.TrimSpace(templateID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &Error{
				Code:    "platform.template_not_found",
				Message: "render template was not found",
			}
		}
		return nil, fmt.Errorf("list render template versions %s: %w", templateID, err)
	}
	return items, nil
}

func (s *Service) UpdateTemplateSource(ctx context.Context, templateID, baseRevisionID, message string, source TemplateSource) (TemplateDetail, error) {
	templateID = strings.TrimSpace(templateID)
	baseRevisionID = strings.TrimSpace(baseRevisionID)
	message = strings.TrimSpace(message)

	bundle, compiled, validation, err := s.validateTemplateForWrite(ctx, templateID, source)
	if err != nil {
		return TemplateDetail{}, err
	}

	savedAt := time.Now().UTC().Format(time.RFC3339Nano)
	revision := newStoredRevision(templateID, newRevisionID(templateID, bundle.digest), compiled, "save", &message, savedAt)
	if err := s.templateRepo.SaveCurrentRevision(ctx, templateID, baseRevisionID, revision, validation); err != nil {
		return TemplateDetail{}, s.mapTemplateWriteError(err)
	}

	return s.GetTemplate(ctx, templateID)
}

func (s *Service) RollbackTemplate(ctx context.Context, templateID, targetRevisionID, baseRevisionID, message string) (TemplateDetail, error) {
	templateID = strings.TrimSpace(templateID)
	targetRevisionID = strings.TrimSpace(targetRevisionID)
	baseRevisionID = strings.TrimSpace(baseRevisionID)
	message = strings.TrimSpace(message)

	state, _, err := s.templateRepo.loadCurrentTemplate(ctx, templateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TemplateDetail{}, &Error{
				Code:    "platform.template_not_found",
				Message: "render template was not found",
			}
		}
		return TemplateDetail{}, fmt.Errorf("get render template state %s: %w", templateID, err)
	}
	if state.CurrentRevisionID != baseRevisionID {
		return TemplateDetail{}, &Error{
			Code:    "platform.template_revision_conflict",
			Message: "render template revision is stale",
		}
	}
	if targetRevisionID == state.CurrentRevisionID {
		return TemplateDetail{}, &Error{
			Code:    "platform.template_rollback_target_invalid",
			Message: "render template rollback target is invalid",
		}
	}

	targetSource, err := s.templateRepo.GetRevisionSource(ctx, templateID, targetRevisionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TemplateDetail{}, &Error{
				Code:    "platform.template_revision_not_found",
				Message: "render template revision was not found",
			}
		}
		return TemplateDetail{}, fmt.Errorf("get render template rollback source %s/%s: %w", templateID, targetRevisionID, err)
	}

	bundle, compiled, validation, err := s.validateTemplateForWrite(ctx, templateID, targetSource)
	if err != nil {
		var renderErr *Error
		if errors.As(err, &renderErr) && renderErr.Code == "platform.template_source_invalid" {
			return TemplateDetail{}, &Error{
				Code:    "platform.template_rollback_target_invalid",
				Message: "render template rollback target is invalid",
			}
		}
		return TemplateDetail{}, err
	}

	savedAt := time.Now().UTC().Format(time.RFC3339Nano)
	revision := newStoredRevision(templateID, newRevisionID(templateID, bundle.digest), compiled, "rollback", &message, savedAt)
	if err := s.templateRepo.SaveCurrentRevision(ctx, templateID, baseRevisionID, revision, validation); err != nil {
		return TemplateDetail{}, s.mapTemplateWriteError(err)
	}

	return s.GetTemplate(ctx, templateID)
}

func (s *Service) ArtifactURL(artifactID string) string {
	return "/api/system/render/artifacts/" + artifactID
}

func (s *Service) LookupArtifact(artifactID string) (Artifact, error) {
	if s == nil {
		return Artifact{}, &Error{Code: "platform.resource_missing", Message: "render service is not available"}
	}
	if !artifactIDPattern.MatchString(strings.TrimSpace(artifactID)) {
		return Artifact{}, &Error{Code: "platform.resource_missing", Message: "render artifact was not found"}
	}

	s.mu.RLock()
	if artifact, ok := s.artifacts[artifactID]; ok {
		s.mu.RUnlock()
		return artifact, nil
	}
	s.mu.RUnlock()

	recordPath := filepath.Join(s.outputRoot, artifactID+".json")
	recordBytes, err := os.ReadFile(recordPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Artifact{}, &Error{Code: "platform.resource_missing", Message: "render artifact was not found", Err: err}
		}
		return Artifact{}, fmt.Errorf("read render artifact record %s: %w", recordPath, err)
	}

	var record artifactRecord
	if err := json.Unmarshal(recordBytes, &record); err != nil {
		return Artifact{}, fmt.Errorf("decode render artifact record %s: %w", recordPath, err)
	}

	artifactPath := filepath.Join(s.outputRoot, filepath.Base(record.Filename))
	if !pathWithinRoot(s.outputRoot, artifactPath) {
		return Artifact{}, &Error{Code: "platform.resource_missing", Message: "render artifact path is invalid"}
	}
	if _, err := os.Stat(artifactPath); err != nil {
		if os.IsNotExist(err) {
			return Artifact{}, &Error{Code: "platform.resource_missing", Message: "render artifact was not found", Err: err}
		}
		return Artifact{}, fmt.Errorf("inspect render artifact %s: %w", artifactPath, err)
	}

	artifact := Artifact{
		ArtifactID: record.ArtifactID,
		MIME:       record.MIME,
		Path:       artifactPath,
	}

	s.mu.Lock()
	s.artifacts[artifactID] = artifact
	s.mu.Unlock()

	return artifact, nil
}

func (s *Service) Diagnostics() []health.DiagnosticIssue {
	issues := make([]health.DiagnosticIssue, 0, 4)

	info, err := os.Stat(s.templatesRoot)
	switch {
	case os.IsNotExist(err):
		issues = append(issues, health.DiagnosticIssue{
			Code:        "platform.resource_missing",
			Severity:    "warning",
			Summary:     "模板资源目录缺失",
			Remediation: "请恢复仓库中的 templates 目录。",
		})
	case err != nil:
		issues = append(issues, health.DiagnosticIssue{
			Code:        "platform.resource_missing",
			Severity:    "warning",
			Summary:     "模板资源目录不可读",
			Remediation: "请确认 templates 目录存在且当前进程有读取权限。",
		})
	case !info.IsDir():
		issues = append(issues, health.DiagnosticIssue{
			Code:        "platform.resource_missing",
			Severity:    "warning",
			Summary:     "模板资源目录结构无效",
			Remediation: "请恢复仓库中的 templates 目录结构。",
		})
	default:
		templateSeeds, err := discoverTemplateSeeds(s.templatesRoot, s.logger)
		if err != nil {
			issues = append(issues, health.DiagnosticIssue{
				Code:        "platform.resource_missing",
				Severity:    "warning",
				Summary:     "模板资源目录不可读",
				Remediation: "请确认 templates 目录存在且当前进程有读取权限。",
			})
			break
		}
		required := []string{"help.menu", "status.panel"}
		for _, templateID := range required {
			if _, ok := templateSeeds[templateID]; ok {
				continue
			}
			issues = append(issues, health.DiagnosticIssue{
				Code:        "platform.resource_missing",
				Severity:    "warning",
				Summary:     fmt.Sprintf("渲染模板 %s 缺失", templateID),
				Remediation: "请恢复仓库中的正式模板资源。",
			})
		}
	}

	if strings.TrimSpace(s.browserPath) != "" {
		return issues
	}

	inspection, err := deps.NewManager(s.repoRoot).Inspect("chromium")
	if err != nil {
		var bootstrapErr *deps.BootstrapError
		if errors.As(err, &bootstrapErr) {
			issues = append(issues, health.DiagnosticIssue{
				Code:        "platform.resource_missing",
				Severity:    "warning",
				Summary:     bootstrapErr.Message,
				Remediation: bootstrapErr.Remediation,
			})
			return issues
		}
		issues = append(issues, health.DiagnosticIssue{
			Code:        "platform.resource_missing",
			Severity:    "warning",
			Summary:     "Chromium 资源清单不可用。",
			Remediation: "请恢复 .deps/manifest.json，或在配置中显式设置 render.browser_path。",
		})
		return issues
	}
	if !inspection.MetadataComplete {
		issues = append(issues, health.DiagnosticIssue{
			Code:        "platform.resource_missing",
			Severity:    "warning",
			Summary:     deps.BootstrapSummary("chromium", inspection),
			Remediation: "请恢复当前平台 Chromium 资源的 archive_format、entrypoints、来源列表与 sha256，或在配置中显式设置 render.browser_path。",
		})
		return issues
	}
	if inspection.PreparedStorePresent {
		return issues
	}
	if inspection.CachedArchivePresent {
		issues = append(issues, health.DiagnosticIssue{
			Code:        "platform.resource_missing",
			Severity:    "warning",
			Summary:     "Chromium 资源归档已缓存，仍需展开运行时。",
			Remediation: deps.BootstrapRemediation("chromium", inspection.ArchivePath, inspection.StoreRoot),
		})
		return issues
	}
	issues = append(issues, health.DiagnosticIssue{
		Code:        "platform.resource_missing",
		Severity:    "warning",
		Summary:     "Chromium 资源尚未准备完成。",
		Remediation: deps.BootstrapRemediation("chromium", inspection.ArchivePath, inspection.StoreRoot),
	})
	return issues
}

func (s *Service) normalizeRequest(request Request) (Request, []byte, error) {
	request.Template = strings.TrimSpace(request.Template)
	request.Theme = strings.TrimSpace(request.Theme)
	request.Output = strings.ToLower(strings.TrimSpace(request.Output))

	if request.Template == "" {
		return Request{}, nil, &Error{Code: "platform.invalid_request", Message: "render template is required"}
	}
	if request.Theme == "" {
		request.Theme = "default"
	}
	switch request.Output {
	case "", "png":
		request.Output = "png"
	case "jpeg":
	default:
		return Request{}, nil, &Error{Code: "platform.invalid_request", Message: "render output must be png or jpeg"}
	}
	if request.Data == nil {
		request.Data = map[string]any{}
	}
	request.Data = cloneRenderData(request.Data)
	request.Data["render_footer"] = s.renderFooter(request.Plugin)

	payloadBytes, err := json.Marshal(request.Data)
	if err != nil {
		return Request{}, nil, &Error{Code: "platform.invalid_request", Message: "render data is not serializable", Err: err}
	}
	if len(payloadBytes) > s.currentMaxRenderDataBytes() {
		return Request{}, nil, &Error{
			Code:    "platform.render_input_too_large",
			Message: "render input exceeds the configured size limit",
		}
	}

	return request, payloadBytes, nil
}

func (s *Service) reserveSlot() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	limit := s.workerCount + s.queueMaxLength
	if limit <= 0 {
		limit = s.workerCount
	}
	if s.activeRequests >= limit {
		return &Error{
			Code:    "platform.render_queue_full",
			Message: "render queue is full",
		}
	}
	s.activeRequests++
	return nil
}

func (s *Service) releaseSlot() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeRequests > 0 {
		s.activeRequests--
	}
}

func (s *Service) currentQueueWaitTimeout() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.queueWaitTimeout
}

func (s *Service) currentRenderTimeout() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.renderTimeout
}

func (s *Service) currentMaxRenderDataBytes() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.maxRenderDataBytes
}

func (s *Service) currentFooterTemplate() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if strings.TrimSpace(s.footerTemplate) == "" {
		return defaultRenderFooter
	}
	return s.footerTemplate
}

func (s *Service) renderFooter(plugin *PluginContext) string {
	pluginName := systemTemplatePlugin
	pluginVersion := developmentVersion
	if plugin != nil {
		if name := strings.TrimSpace(plugin.Name); name != "" {
			pluginName = name
		}
		if version := displayVersion(plugin.Version); version != "" {
			pluginVersion = version
		}
	}

	replacer := strings.NewReplacer(
		"{{rayleabot_version}}", displayVersion(detectRenderCoreVersion(s.repoRoot)),
		"{{plugin_name}}", pluginName,
		"{{plugin_version}}", pluginVersion,
	)
	return replacer.Replace(s.currentFooterTemplate())
}

func displayVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" || version == "0.0.0-dev" {
		return developmentVersion
	}
	return version
}

func detectRenderCoreVersion(repoRoot string) string {
	content, err := os.ReadFile(filepath.Join(repoRoot, "build_info.json"))
	if err != nil {
		return developmentVersion
	}
	var payload struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(content, &payload); err != nil {
		return developmentVersion
	}
	return displayVersion(payload.Version)
}

func cloneRenderData(data map[string]any) map[string]any {
	cloned := make(map[string]any, len(data)+1)
	for key, value := range data {
		cloned[key] = value
	}
	return cloned
}

func (s *Service) cachedResult(cacheKey string) (Result, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result, ok := s.cache[cacheKey]
	return result, ok
}

func (s *Service) persistArtifact(request Request, cacheKey string, content []byte) (Result, error) {
	artifactID := buildArtifactID(cacheKey)
	filename := artifactID + outputSuffix(request.Output)
	artifactPath := filepath.Join(s.outputRoot, filename)
	if err := os.WriteFile(artifactPath, content, 0o644); err != nil {
		return Result{}, fmt.Errorf("write render artifact %s: %w", artifactPath, err)
	}

	record := artifactRecord{
		ArtifactID: artifactID,
		CacheKey:   cacheKey,
		Template:   request.Template,
		Theme:      request.Theme,
		Output:     request.Output,
		MIME:       outputMIME(request.Output),
		Filename:   filename,
	}
	recordBytes, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return Result{}, fmt.Errorf("encode render artifact record %s: %w", artifactID, err)
	}
	if err := os.WriteFile(filepath.Join(s.outputRoot, artifactID+".json"), recordBytes, 0o644); err != nil {
		return Result{}, fmt.Errorf("write render artifact record %s: %w", artifactID, err)
	}

	result := Result{
		ArtifactID: artifactID,
		ImagePath:  fileURL(artifactPath),
		MIME:       record.MIME,
		CacheKey:   cacheKey,
		Template:   request.Template,
		Theme:      request.Theme,
		FromCache:  false,
	}

	s.mu.Lock()
	s.artifacts[artifactID] = Artifact{
		ArtifactID: artifactID,
		MIME:       record.MIME,
		Path:       artifactPath,
	}
	s.mu.Unlock()

	return result, nil
}

func (s *Service) loadArtifacts() error {
	entries, err := os.ReadDir(s.outputRoot)
	if err != nil {
		return fmt.Errorf("read render output root %s: %w", s.outputRoot, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		recordPath := filepath.Join(s.outputRoot, entry.Name())
		recordBytes, err := os.ReadFile(recordPath)
		if err != nil {
			return fmt.Errorf("read render artifact record %s: %w", recordPath, err)
		}

		var record artifactRecord
		if err := json.Unmarshal(recordBytes, &record); err != nil {
			return fmt.Errorf("decode render artifact record %s: %w", recordPath, err)
		}

		artifactPath := filepath.Join(s.outputRoot, filepath.Base(record.Filename))
		if !pathWithinRoot(s.outputRoot, artifactPath) {
			continue
		}
		if _, err := os.Stat(artifactPath); err != nil {
			continue
		}

		result := Result{
			ArtifactID: record.ArtifactID,
			ImagePath:  fileURL(artifactPath),
			MIME:       record.MIME,
			CacheKey:   record.CacheKey,
			Template:   record.Template,
			Theme:      record.Theme,
			FromCache:  true,
		}
		s.cache[record.CacheKey] = result
		s.artifacts[record.ArtifactID] = Artifact{
			ArtifactID: record.ArtifactID,
			MIME:       record.MIME,
			Path:       artifactPath,
		}
	}

	return nil
}

func (s *Service) resolveCompiledTemplate(ctx context.Context, request Request) (*compiledTemplate, string, string, error) {
	_, source, err := s.templateRepo.GetCurrentSource(ctx, request.Template)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", "", &Error{
				Code:    "platform.template_not_found",
				Message: "render template was not found",
			}
		}
		return nil, "", "", fmt.Errorf("get current render template %s: %w", request.Template, err)
	}

	bundle, err := buildTemplateSourceBundle(request.Template, source)
	if err != nil {
		return nil, "", "", &Error{
			Code:    "platform.internal_error",
			Message: "stored render template is invalid",
			Err:     err,
		}
	}
	compiled, issues, err := compileTemplateBundle(bundle)
	if err != nil {
		return nil, "", "", fmt.Errorf("compile current render template %s: %w", request.Template, err)
	}
	if len(issues) > 0 {
		return nil, "", "", &Error{
			Code:    "platform.internal_error",
			Message: "stored render template is invalid",
		}
	}
	return compiled, compiled.bundle.manifest.Version, compiled.bundle.digest, nil
}

func (s *Service) validateTemplateForWrite(ctx context.Context, templateID string, source TemplateSource) (templateSourceBundle, *compiledTemplate, TemplateValidationStatus, error) {
	if exists, err := s.templateRepo.templateExists(ctx, templateID); err != nil {
		return templateSourceBundle{}, nil, TemplateValidationStatus{}, fmt.Errorf("query render template %s: %w", templateID, err)
	} else if !exists {
		return templateSourceBundle{}, nil, TemplateValidationStatus{}, &Error{
			Code:    "platform.template_not_found",
			Message: "render template was not found",
		}
	}

	bundle, err := buildTemplateSourceBundle(templateID, source)
	if err != nil {
		_ = s.templateRepo.UpdateValidationStatus(ctx, templateID, newValidationStatus(false, 1))
		return templateSourceBundle{}, nil, TemplateValidationStatus{}, err
	}

	compiled, issues, err := compileTemplateBundle(bundle)
	if err != nil {
		return templateSourceBundle{}, nil, TemplateValidationStatus{}, fmt.Errorf("compile render template %s: %w", templateID, err)
	}

	validation := newValidationStatus(len(issues) == 0, len(issues))
	if err := s.templateRepo.UpdateValidationStatus(ctx, templateID, validation); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return templateSourceBundle{}, nil, TemplateValidationStatus{}, fmt.Errorf("update render template validation %s: %w", templateID, err)
	}
	if len(issues) > 0 {
		return templateSourceBundle{}, nil, TemplateValidationStatus{}, &Error{
			Code:    "platform.template_source_invalid",
			Message: issues[0].Message,
		}
	}

	return bundle, compiled, validation, nil
}

func (s *Service) syncTemplatesFromFiles(ctx context.Context) error {
	if s == nil {
		return nil
	}

	s.templateSyncMu.Lock()
	defer s.templateSyncMu.Unlock()

	templateSeeds, err := discoverTemplateSeeds(s.templatesRoot, s.logger)
	if err != nil {
		return err
	}

	for _, templateID := range sortedTemplateIDs(templateSeeds) {
		seed := templateSeeds[templateID]
		templateDir := filepath.Join(s.templatesRoot, filepath.Clean(templateID))
		if err := s.syncTemplateSeed(ctx, templateID, seed, TemplateSourceInfo{Type: "system"}, templateDir); err != nil {
			return fmt.Errorf("sync render template %s: %w", templateID, err)
		}
	}
	return nil
}

func (s *Service) SyncPluginTemplates(ctx context.Context, sources []PluginTemplateSource) error {
	if s == nil {
		return nil
	}

	s.templateSyncMu.Lock()
	defer s.templateSyncMu.Unlock()

	keepByPlugin := map[string][]string{}
	seenTemplates := map[string]struct{}{}
	for _, source := range sources {
		pluginID := strings.TrimSpace(source.PluginID)
		dir := strings.TrimSpace(source.Dir)
		if pluginID == "" || dir == "" {
			continue
		}
		seed, err := loadTemplateSeed(dir)
		if err != nil {
			return fmt.Errorf("load plugin render template %s: %w", pluginID, err)
		}
		localID := strings.TrimSpace(seed.compiled.bundle.manifest.ID)
		if !pluginTemplateLocalIDPattern.MatchString(localID) {
			return fmt.Errorf("plugin render template %s has invalid local id %q", pluginID, localID)
		}
		templateID := formalPluginTemplateID(pluginID, localID)
		if _, ok := seenTemplates[templateID]; ok {
			return fmt.Errorf("duplicate plugin render template id %s", templateID)
		}
		seenTemplates[templateID] = struct{}{}
		seed.source.ManifestJSON["id"] = templateID
		seed.compiled.bundle.manifest.ID = templateID
		seed.compiled.bundle.normalizedManifest["id"] = templateID
		seed.compiled.bundle.source.ManifestJSON["id"] = templateID
		seed.compiled.bundle.digest = digestTemplateSource(seed.compiled.bundle.source)
		seed.source = seed.compiled.bundle.source
		if err := s.syncTemplateSeed(ctx, templateID, seed, TemplateSourceInfo{
			Type:     "plugin",
			PluginID: pluginID,
			LocalID:  localID,
		}, dir); err != nil {
			return fmt.Errorf("sync plugin render template %s/%s: %w", pluginID, localID, err)
		}
		keepByPlugin[pluginID] = append(keepByPlugin[pluginID], templateID)
	}

	for pluginID, keepIDs := range keepByPlugin {
		if err := s.templateRepo.RemovePluginTemplatesExcept(ctx, pluginID, keepIDs); err != nil {
			return err
		}
	}
	activePluginIDs := make([]string, 0, len(keepByPlugin))
	for pluginID := range keepByPlugin {
		activePluginIDs = append(activePluginIDs, pluginID)
	}
	if err := s.templateRepo.RemovePluginTemplatesNotIn(ctx, activePluginIDs); err != nil {
		return err
	}
	return nil
}

func ValidatePluginTemplateSources(sources []PluginTemplateSource) error {
	seenTemplates := map[string]struct{}{}
	for _, source := range sources {
		pluginID := strings.TrimSpace(source.PluginID)
		dir := strings.TrimSpace(source.Dir)
		if pluginID == "" || dir == "" {
			return fmt.Errorf("plugin render template declaration is incomplete")
		}
		seed, err := loadTemplateSeed(dir)
		if err != nil {
			return fmt.Errorf("load plugin render template %s: %w", pluginID, err)
		}
		localID := strings.TrimSpace(seed.compiled.bundle.manifest.ID)
		if !pluginTemplateLocalIDPattern.MatchString(localID) {
			return fmt.Errorf("plugin render template %s has invalid local id %q", pluginID, localID)
		}
		templateID := formalPluginTemplateID(pluginID, localID)
		if _, ok := seenTemplates[templateID]; ok {
			return fmt.Errorf("duplicate plugin render template id %s", templateID)
		}
		seenTemplates[templateID] = struct{}{}
	}
	return nil
}

func PluginTemplateSourcesFromManifests(items []PluginTemplateSource) []PluginTemplateSource {
	sources := make([]PluginTemplateSource, 0, len(items))
	for _, item := range items {
		pluginID := strings.TrimSpace(item.PluginID)
		dir := strings.TrimSpace(item.Dir)
		if pluginID == "" || dir == "" {
			continue
		}
		seed, err := loadTemplateSeed(dir)
		if err != nil {
			continue
		}
		localID := strings.TrimSpace(seed.compiled.bundle.manifest.ID)
		if !pluginTemplateLocalIDPattern.MatchString(localID) {
			continue
		}
		item.PluginID = pluginID
		item.LocalID = localID
		item.Dir = dir
		sources = append(sources, item)
	}
	return sources
}

func (s *Service) RemovePluginTemplates(ctx context.Context, pluginID string) error {
	if s == nil {
		return nil
	}
	if err := s.templateRepo.RemovePluginTemplatesExcept(ctx, pluginID, nil); err != nil {
		return err
	}

	prefix := "plugin." + strings.TrimSpace(pluginID) + "."
	s.mu.Lock()
	defer s.mu.Unlock()
	for templateID := range s.templateRoots {
		if strings.HasPrefix(templateID, prefix) {
			delete(s.templateRoots, templateID)
		}
	}
	return nil
}

func (s *Service) syncTemplateSeed(ctx context.Context, templateID string, seed templateSeed, sourceInfo TemplateSourceInfo, templateDir string) error {
	savedAt := time.Now().UTC().Format(time.RFC3339Nano)
	revision := newStoredRevision(
		templateID,
		newRevisionID(templateID, seed.compiled.bundle.digest),
		seed.compiled,
		"save",
		nil,
		savedAt,
	)
	changed, err := s.templateRepo.SyncTemplateRevision(ctx, revision, TemplateValidationStatus{
		Valid:      true,
		CheckedAt:  savedAt,
		IssueCount: 0,
	}, sourceInfo)
	if err != nil {
		return err
	}

	s.rememberTemplateDir(templateID, templateDir)
	if changed && s.logger != nil {
		s.logger.Info(
			"render template synchronized",
			"component", "render",
			"template_id", templateID,
			"revision_id", revision.RevisionID,
			"source_digest", revision.SourceDigest,
		)
	}
	return nil
}

func (s *Service) mapTemplateWriteError(err error) error {
	var renderErr *Error
	if errors.As(err, &renderErr) {
		return renderErr
	}
	if errors.Is(err, sql.ErrNoRows) {
		return &Error{
			Code:    "platform.template_not_found",
			Message: "render template was not found",
		}
	}
	return fmt.Errorf("write render template revision: %w", err)
}

func newStoredRevision(templateID, revisionID string, compiled *compiledTemplate, kind string, message *string, savedAt string) storedTemplateRevision {
	manifestJSON, _ := json.Marshal(compiled.bundle.normalizedManifest)
	inputSchemaJSON := sql.NullString{}
	if compiled.bundle.source.InputSchemaJSON != nil {
		encoded, _ := json.Marshal(compiled.bundle.source.InputSchemaJSON)
		inputSchemaJSON = sql.NullString{String: string(encoded), Valid: true}
	}

	return storedTemplateRevision{
		RevisionID:      revisionID,
		TemplateID:      templateID,
		TemplateVersion: compiled.bundle.manifest.Version,
		Kind:            kind,
		Message:         message,
		SavedAt:         savedAt,
		SourceDigest:    compiled.bundle.digest,
		ManifestJSON:    string(manifestJSON),
		HTML:            compiled.bundle.source.HTML,
		Stylesheet:      compiled.bundle.source.Stylesheet,
		InputSchemaJSON: inputSchemaJSON,
	}
}

func newRevisionID(templateID, digest string) string {
	templateID = strings.NewReplacer(".", "_", "-", "_", "/", "_").Replace(strings.TrimSpace(templateID))
	if len(digest) > 8 {
		digest = digest[:8]
	}
	sequence := atomic.AddUint64(&revisionCounter, 1)
	return fmt.Sprintf("rev_%s_%s_%s_%06d", templateID, time.Now().UTC().Format("20060102T150405000000000"), digest, sequence)
}

func newValidationStatus(valid bool, issueCount int) TemplateValidationStatus {
	return TemplateValidationStatus{
		Valid:      valid,
		CheckedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		IssueCount: issueCount,
	}
}

func issuesOrEmpty(issues []TemplateValidationIssue) []TemplateValidationIssue {
	if len(issues) == 0 {
		return []TemplateValidationIssue{}
	}
	return issues
}

func buildCacheKey(request Request, version string, sourceDigest string, resourceDigest string, payloadBytes []byte) string {
	sum := sha256.Sum256(payloadBytes)
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s:%s", renderCacheVersion, request.Template, version, sourceDigest, resourceDigest, request.Theme, request.Output, hex.EncodeToString(sum[:12]))
}

func buildArtifactID(cacheKey string) string {
	sum := sha256.Sum256([]byte(cacheKey))
	return "artifact_" + hex.EncodeToString(sum[:12])
}

func outputSuffix(output string) string {
	switch output {
	case "jpeg":
		return ".jpg"
	default:
		return ".png"
	}
}

func outputMIME(output string) string {
	switch output {
	case "jpeg":
		return "image/jpeg"
	default:
		return "image/png"
	}
}

func fileURL(path string) string {
	return (&url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(path),
	}).String()
}

func (s *Service) rememberTemplateDir(templateID, templateDir string) {
	if s == nil || strings.TrimSpace(templateID) == "" || strings.TrimSpace(templateDir) == "" {
		return
	}
	absolute, err := filepath.Abs(templateDir)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.templateRoots[strings.TrimSpace(templateID)] = absolute
}

func (s *Service) templateDirFor(templateID string) string {
	if s == nil {
		return ""
	}
	templateID = strings.TrimSpace(templateID)
	s.mu.RLock()
	if dir := s.templateRoots[templateID]; dir != "" {
		s.mu.RUnlock()
		return dir
	}
	s.mu.RUnlock()
	return filepath.Join(s.templatesRoot, filepath.Clean(templateID))
}

func templateBaseURL(templateDir string) string {
	templateDir, err := filepath.Abs(templateDir)
	if err != nil || templateDir == "" {
		return ""
	}
	path := filepath.ToSlash(templateDir)
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return (&url.URL{
		Scheme: "file",
		Path:   path,
	}).String()
}

func templateResourceDigest(templateDir string) string {
	templateDir, err := filepath.Abs(templateDir)
	if err != nil || templateDir == "" {
		return ""
	}
	assetsDir := filepath.Join(templateDir, "assets")
	if !pathWithinRoot(templateDir, assetsDir) {
		return ""
	}

	digest := sha256.New()
	walkErr := filepath.WalkDir(assetsDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(templateDir, path)
		if err != nil {
			return err
		}
		digest.Write([]byte(filepath.ToSlash(relative)))
		digest.Write([]byte{0})
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		digest.Write(content)
		digest.Write([]byte{0})
		return nil
	})
	if walkErr != nil {
		return ""
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func formalPluginTemplateID(pluginID, localID string) string {
	pluginID = strings.TrimSpace(pluginID)
	localID = strings.Trim(filepath.ToSlash(strings.TrimSpace(localID)), "/")
	if pluginID == "" || localID == "" {
		return ""
	}
	return "plugin." + pluginID + "." + localID
}

func parseFormalPluginTemplateID(templateID string) (string, string, bool) {
	templateID = strings.TrimSpace(templateID)
	const prefix = "plugin."
	if !strings.HasPrefix(templateID, prefix) {
		return "", "", false
	}
	remainder := strings.TrimPrefix(templateID, prefix)
	separator := strings.LastIndex(remainder, ".")
	if separator <= 0 || separator == len(remainder)-1 {
		return "", "", false
	}
	pluginID := strings.TrimSpace(remainder[:separator])
	localID := strings.TrimSpace(remainder[separator+1:])
	if pluginID == "" || !pluginTemplateLocalIDPattern.MatchString(localID) {
		return "", "", false
	}
	return pluginID, localID, true
}

func pathWithinRoot(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func wrapRenderError(err error, message string) error {
	var renderErr *Error
	if errors.As(err, &renderErr) {
		return renderErr
	}
	return &Error{
		Code:    "platform.internal_error",
		Message: message,
		Err:     err,
	}
}
