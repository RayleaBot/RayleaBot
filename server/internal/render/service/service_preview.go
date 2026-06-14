package service

import "context"

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
	if cached, ok := s.cachedPreviewHTML(cacheKey); ok {
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
	s.cachePreviewHTML(cacheKey, preview)
	return preview, nil
}
