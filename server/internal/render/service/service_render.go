package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	renderartifact "github.com/RayleaBot/RayleaBot/server/internal/render/artifact"
	renderbrowser "github.com/RayleaBot/RayleaBot/server/internal/render/browser"
	renderworker "github.com/RayleaBot/RayleaBot/server/internal/render/engine"
	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
)

func (s *Service) Render(ctx context.Context, request Request) (renderartifact.Result, error) {
	if s == nil {
		return renderartifact.Result{}, &rendertemplates.Error{Code: "platform.resource_missing", Message: "render service is not available"}
	}

	startedAt := time.Now()
	result, err := s.renderInternal(ctx, request)
	s.recordRenderMetric(renderOutcome(result, err), time.Since(startedAt))
	return result, err
}

func (s *Service) renderInternal(ctx context.Context, request Request) (renderartifact.Result, error) {
	normalized, payloadBytes, err := s.normalizeRequest(request)
	if err != nil {
		return renderartifact.Result{}, err
	}

	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return renderartifact.Result{}, err
	}

	compiled, _, cacheVersion, cacheDigest, err := s.resolveCompiledTemplate(ctx, normalized)
	if err != nil {
		return renderartifact.Result{}, err
	}
	templateDir := s.templateDirFor(normalized.Template)
	resourceDigest := rendertemplates.ResourceDigest(templateDir)
	deviceScalePercent := s.currentDeviceScalePercent()
	cacheKey := buildCacheKey(normalized, cacheVersion, cacheDigest, resourceDigest, deviceScalePercent, payloadBytes)
	if cached, ok := s.artifactStore.cachedResult(cacheKey); ok {
		cached.FromCache = true
		return cached, nil
	}

	releaseWorker, err := s.worker.Acquire(ctx)
	if err != nil {
		return renderartifact.Result{}, err
	}
	defer releaseWorker()

	if cached, ok := s.artifactStore.cachedResult(cacheKey); ok {
		cached.FromCache = true
		return cached, nil
	}

	html, err := compiled.RenderHTML(normalized.Theme, normalized.Data)
	if err != nil {
		return renderartifact.Result{}, wrapRenderError(err, "render template execution failed")
	}

	renderCtx, cancel := s.worker.RenderContext(ctx)
	defer cancel()

	runner := s.currentRunner()
	if runner == nil {
		return renderartifact.Result{}, &rendertemplates.Error{Code: "platform.resource_missing", Message: "render runner is not available"}
	}
	content, err := runner.Render(renderCtx, renderbrowser.Document{
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
		return renderartifact.Result{}, wrapRenderError(renderworker.WrapRenderError(renderCtx, err), "render execution failed")
	}

	result, err := s.artifactStore.persist(normalized, cacheKey, content)
	if err != nil {
		return renderartifact.Result{}, err
	}

	s.artifactStore.cacheResult(cacheKey, result)

	return result, nil
}

func (s *Service) resolveCompiledTemplate(ctx context.Context, request Request) (*rendertemplates.CompiledTemplate, string, string, string, error) {
	revisionID, source, err := s.templateRepo.GetCurrentSource(ctx, request.Template)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", "", "", &rendertemplates.Error{
				Code:    "platform.template_not_found",
				Message: "render template was not found",
			}
		}
		return nil, "", "", "", fmt.Errorf("get current render template %s: %w", request.Template, err)
	}

	bundle, err := rendertemplates.BuildSourceBundle(request.Template, source)
	if err != nil {
		return nil, "", "", "", &rendertemplates.Error{
			Code:    "platform.internal_error",
			Message: "stored render template is invalid",
			Err:     err,
		}
	}
	compiled, issues, err := rendertemplates.CompileBundle(bundle)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("compile current render template %s: %w", request.Template, err)
	}
	if len(issues) > 0 {
		return nil, "", "", "", &rendertemplates.Error{
			Code:    "platform.internal_error",
			Message: "stored render template is invalid",
		}
	}
	return compiled, revisionID, compiled.Bundle.Manifest.Version, compiled.Bundle.Digest, nil
}

func wrapRenderError(err error, message string) error {
	var renderErr *rendertemplates.Error
	if errors.As(err, &renderErr) {
		return renderErr
	}
	return &rendertemplates.Error{
		Code:    "platform.internal_error",
		Message: message,
		Err:     err,
	}
}
