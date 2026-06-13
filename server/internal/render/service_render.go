package render

import (
	"context"
	"errors"
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
	resourceDigest := templateResourceDigest(templateDir)
	deviceScalePercent := s.currentDeviceScalePercent()
	cacheKey := buildCacheKey(normalized, cacheVersion, cacheDigest, resourceDigest, deviceScalePercent, payloadBytes)
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
		Template:          normalized.Template,
		Theme:             normalized.Theme,
		Output:            normalized.Output,
		BaseURL:           templateBaseURL(templateDir),
		Width:             compiled.bundle.manifest.Width,
		Height:            compiled.bundle.manifest.Height,
		AutoHeight:        true,
		DeviceScaleFactor: deviceScaleFactorFromPercent(deviceScalePercent),
		HTML:              html,
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
