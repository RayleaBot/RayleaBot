package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *Service) Render(ctx context.Context, request Request) (Result, error) {
	if s == nil {
		return Result{}, &Error{Code: "platform.resource_missing", Message: "render service is not available"}
	}

	startedAt := time.Now()
	result, err := s.renderInternal(ctx, request)
	s.recordRenderMetric(renderOutcome(result, err), time.Since(startedAt))
	return result, err
}

func (s *Service) PreviewHTML(ctx context.Context, request Request) (PreviewHTML, error) {
	if s == nil {
		return PreviewHTML{}, &Error{Code: "platform.resource_missing", Message: "render service is not available"}
	}

	normalized, payloadBytes, err := s.normalizeRequest(request)
	if err != nil {
		return PreviewHTML{}, err
	}

	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return PreviewHTML{}, err
	}

	compiled, revisionID, _, _, err := s.resolveCompiledTemplate(ctx, normalized)
	if err != nil {
		return PreviewHTML{}, err
	}
	cacheKey := buildPreviewHTMLCacheKey(normalized, revisionID, payloadBytes)
	if cached, ok := s.artifactStore.cachedPreviewHTML(cacheKey); ok {
		return cached, nil
	}
	html, err := compiled.RenderHTML(normalized.Theme, normalized.Data)
	if err != nil {
		return PreviewHTML{}, wrapRenderError(err, "render template execution failed")
	}

	preview := PreviewHTML{
		TemplateID: normalized.Template,
		RevisionID: revisionID,
		Width:      compiled.Bundle.Manifest.Width,
		Height:     compiled.Bundle.Manifest.Height,
		HTML:       html,
	}
	s.artifactStore.cachePreviewHTML(cacheKey, preview)
	return preview, nil
}

func (s *Service) renderInternal(ctx context.Context, request Request) (Result, error) {
	normalized, payloadBytes, err := s.normalizeRequest(request)
	if err != nil {
		return Result{}, err
	}

	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return Result{}, err
	}

	compiled, _, cacheVersion, cacheDigest, err := s.resolveCompiledTemplate(ctx, normalized)
	if err != nil {
		return Result{}, err
	}
	templateDir := s.templateDirFor(normalized.Template)
	resourceDigest := ResourceDigest(templateDir)
	deviceScalePercent := s.currentDeviceScalePercent()
	cacheKey := buildCacheKey(normalized, cacheVersion, cacheDigest, resourceDigest, deviceScalePercent, payloadBytes)
	if cached, ok := s.artifactStore.cachedResult(cacheKey); ok {
		cached.FromCache = true
		return cached, nil
	}

	releaseWorker, err := s.worker.Acquire(ctx)
	if err != nil {
		return Result{}, err
	}
	defer releaseWorker()

	if cached, ok := s.artifactStore.cachedResult(cacheKey); ok {
		cached.FromCache = true
		return cached, nil
	}

	html, err := compiled.RenderHTML(normalized.Theme, normalized.Data)
	if err != nil {
		return Result{}, wrapRenderError(err, "render template execution failed")
	}

	renderCtx, cancel := s.worker.RenderContext(ctx)
	defer cancel()

	runner := s.currentRunner()
	if runner == nil {
		return Result{}, &Error{Code: "platform.resource_missing", Message: "render runner is not available"}
	}
	content, err := runner.Render(renderCtx, Document{
		Template:          normalized.Template,
		Theme:             normalized.Theme,
		Output:            normalized.Output,
		BaseURL:           BaseURL(templateDir),
		Width:             compiled.Bundle.Manifest.Width,
		Height:            compiled.Bundle.Manifest.Height,
		AutoHeight:        true,
		DeviceScaleFactor: deviceScaleFactorFromPercent(deviceScalePercent),
		HTML:              html,
	})
	if err != nil {
		return Result{}, wrapRenderError(WrapRenderError(renderCtx, err), "render execution failed")
	}

	result, err := s.artifactStore.persist(normalized, cacheKey, content)
	if err != nil {
		return Result{}, err
	}

	s.artifactStore.cacheResult(cacheKey, result)

	return result, nil
}

func (s *Service) resolveCompiledTemplate(ctx context.Context, request Request) (*CompiledTemplate, string, string, string, error) {
	revisionID, source, err := s.templateRepo.GetCurrentSource(ctx, request.Template)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", "", "", &Error{
				Code:    "platform.template_not_found",
				Message: "render template was not found",
			}
		}
		return nil, "", "", "", fmt.Errorf("get current render template %s: %w", request.Template, err)
	}

	bundle, err := BuildSourceBundle(request.Template, source)
	if err != nil {
		return nil, "", "", "", &Error{
			Code:    "platform.internal_error",
			Message: "stored render template is invalid",
			Err:     err,
		}
	}
	compiled, issues, err := CompileBundle(bundle)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("compile current render template %s: %w", request.Template, err)
	}
	if len(issues) > 0 {
		return nil, "", "", "", &Error{
			Code:    "platform.internal_error",
			Message: "stored render template is invalid",
		}
	}
	return compiled, revisionID, compiled.Bundle.Manifest.Version, compiled.Bundle.Digest, nil
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

type TemplateErrorInfo struct {
	Code    string
	Message string
}

func AsTemplateError(err error) (TemplateErrorInfo, bool) {
	var renderErr *Error
	if !errors.As(err, &renderErr) {
		return TemplateErrorInfo{}, false
	}
	return TemplateErrorInfo{
		Code:    renderErr.Code,
		Message: renderErr.Message,
	}, true
}

func (s *Service) TemplateAcceptsRenderIdentity(ctx context.Context, templateID string) bool {
	_, source, err := s.GetTemplateSource(ctx, templateID)
	if err != nil {
		return false
	}
	properties, ok := source.InputSchemaJSON["properties"].(map[string]any)
	if !ok {
		return false
	}
	_, hasUser := properties["user"]
	_, hasPermission := properties["permission"]
	return hasUser && hasPermission
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
	case "":
		request.Output = s.currentDefaultOutput()
	case "png":
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
