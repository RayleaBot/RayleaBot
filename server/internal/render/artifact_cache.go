package render

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func (s *Service) cachedResult(cacheKey string) (Result, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result, ok := s.cache[cacheKey]
	return result, ok
}

func (s *Service) cachedPreviewHTML(cacheKey string) (PreviewHTML, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	preview, ok := s.previewHTMLCache[cacheKey]
	return preview, ok
}

func (s *Service) cachePreviewHTML(cacheKey string, preview PreviewHTML) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.previewHTMLCache[cacheKey] = preview
}

func buildCacheKey(request Request, version string, sourceDigest string, resourceDigest string, deviceScalePercent int, payloadBytes []byte) string {
	sum := sha256.Sum256(payloadBytes)
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s:%d:%s", renderCacheVersion, request.Template, version, sourceDigest, resourceDigest, request.Theme, request.Output, normalizeDeviceScalePercent(deviceScalePercent), hex.EncodeToString(sum[:12]))
}

func buildPreviewHTMLCacheKey(request Request, revisionID string, payloadBytes []byte) string {
	sum := sha256.Sum256(payloadBytes)
	return fmt.Sprintf("preview-html:%s:%s:%s:%s", request.Template, revisionID, request.Theme, hex.EncodeToString(sum[:12]))
}

func buildArtifactID(cacheKey string) string {
	sum := sha256.Sum256([]byte(cacheKey))
	return "artifact_" + hex.EncodeToString(sum[:12])
}
