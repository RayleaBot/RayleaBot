package render

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"rayleabot/server/internal/deps"
	"rayleabot/server/internal/health"
)

const (
	defaultWorkerCount      = 1
	defaultQueueMaxLength   = 32
	defaultQueueWaitTimeout = 15 * time.Second
	defaultRenderTimeout    = 20 * time.Second
	defaultRenderDataLimit  = 1 << 20
)

var artifactIDPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

type Runner interface {
	Render(ctx context.Context, doc Document) ([]byte, error)
}

type Options struct {
	RepoRoot           string
	OutputRoot         string
	Runner             Runner
	WorkerCount        int
	BrowserArgs        []string
	BrowserPath        string
	QueueMaxLength     int
	QueueWaitTimeout   time.Duration
	RenderTimeout      time.Duration
	MaxRenderDataBytes int
}

type RuntimeConfig struct {
	QueueMaxLength   int
	QueueWaitTimeout time.Duration
	RenderTimeout    time.Duration
}

type Request struct {
	Template string         `json:"template"`
	Theme    string         `json:"theme,omitempty"`
	Output   string         `json:"output,omitempty"`
	Data     map[string]any `json:"data"`
}

type Document struct {
	Template string
	Theme    string
	Output   string
	Width    int
	Height   int
	HTML     string
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
	repoRoot      string
	templatesRoot string
	outputRoot    string
	browserPath   string
	runner        Runner
	workerSem     chan struct{}
	workerCount   int

	mu                 sync.RWMutex
	queueMaxLength     int
	queueWaitTimeout   time.Duration
	renderTimeout      time.Duration
	maxRenderDataBytes int
	activeRequests     int
	templates          map[string]*renderTemplate
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

	templatesRoot := filepath.Join(repoRoot, "templates")
	templates, err := discoverTemplates(templatesRoot)
	if err != nil {
		return nil, err
	}

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
		runner:             runner,
		workerSem:          make(chan struct{}, workerCount),
		workerCount:        workerCount,
		queueMaxLength:     queueMaxLength,
		queueWaitTimeout:   queueWaitTimeout,
		renderTimeout:      renderTimeout,
		maxRenderDataBytes: maxRenderDataBytes,
		templates:          templates,
		cache:              map[string]Result{},
		artifacts:          map[string]Artifact{},
	}
	if err := service.loadArtifacts(); err != nil {
		return nil, err
	}

	return service, nil
}

func (s *Service) Close() error {
	return nil
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
}

func (s *Service) Render(ctx context.Context, request Request) (Result, error) {
	if s == nil {
		return Result{}, &Error{Code: "platform.resource_missing", Message: "render service is not available"}
	}

	normalized, payloadBytes, err := s.normalizeRequest(request)
	if err != nil {
		return Result{}, err
	}

	tpl, err := s.lookupTemplate(normalized.Template)
	if err != nil {
		return Result{}, err
	}
	cacheKey := buildCacheKey(normalized, tpl.Version, payloadBytes)
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

	html, err := tpl.renderHTML(normalized.Theme, normalized.Data)
	if err != nil {
		return Result{}, wrapRenderError(err, "render template execution failed")
	}

	renderCtx := ctx
	if timeout := s.currentRenderTimeout(); timeout > 0 {
		var cancel context.CancelFunc
		renderCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	content, err := s.runner.Render(renderCtx, Document{
		Template: normalized.Template,
		Theme:    normalized.Theme,
		Output:   normalized.Output,
		Width:    tpl.Width,
		Height:   tpl.Height,
		HTML:     html,
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
		required := []string{"help.menu", "status.panel"}
		for _, templateID := range required {
			if _, ok := s.templates[templateID]; ok {
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

	manifest, err := deps.LoadManifest(s.repoRoot)
	if err != nil {
		issues = append(issues, health.DiagnosticIssue{
			Code:        "platform.resource_missing",
			Severity:    "warning",
			Summary:     "Chromium 资源清单缺失",
			Remediation: "请恢复 .deps/manifest.json 并包含当前平台的 Chromium 资源声明。",
		})
		return issues
	}

	currentPlatform := deps.CurrentPlatform()
	resource := manifest.FindResource(currentPlatform, "chromium")
	if resource == nil {
		issues = append(issues, health.DiagnosticIssue{
			Code:        "platform.resource_missing",
			Severity:    "warning",
			Summary:     fmt.Sprintf("当前平台 %s 缺少 Chromium 资源声明", currentPlatform),
			Remediation: "请在 .deps/manifest.json 中补齐当前平台 Chromium 资源元数据。",
		})
		return issues
	}
	if !deps.ResourceMetadataComplete(resource) {
		issues = append(issues, health.DiagnosticIssue{
			Code:        "platform.resource_missing",
			Severity:    "warning",
			Summary:     fmt.Sprintf("当前平台 %s 的 Chromium 资源元数据不完整", currentPlatform),
			Remediation: "请在 .deps/manifest.json 中补齐当前平台 Chromium 资源的 archive_format、entrypoints、source 与 sha256。",
		})
		return issues
	}
	if _, err := deps.NewManager(s.repoRoot).ResolvePreparedEntrypoint("chromium", "browser"); err != nil && strings.TrimSpace(s.browserPath) == "" {
		issues = append(issues, health.DiagnosticIssue{
			Code:        "platform.resource_missing",
			Severity:    "warning",
			Summary:     "Chromium 资源尚未准备完成",
			Remediation: "请先准备受控 Chromium 运行时，或在配置中显式设置 render.browser_path。",
		})
	}
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

func (s *Service) lookupTemplate(templateID string) (*renderTemplate, error) {
	s.mu.RLock()
	templateEntry, ok := s.templates[templateID]
	s.mu.RUnlock()
	if ok {
		return templateEntry, nil
	}
	return nil, &Error{
		Code:    "platform.resource_missing",
		Message: "render template was not found",
	}
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

func buildCacheKey(request Request, version string, payloadBytes []byte) string {
	sum := sha256.Sum256(payloadBytes)
	return fmt.Sprintf("%s:%s:%s:%s:%s", request.Template, version, request.Theme, request.Output, hex.EncodeToString(sum[:12]))
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
