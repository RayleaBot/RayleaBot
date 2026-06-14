package service

import (
	"errors"

	renderartifact "github.com/RayleaBot/RayleaBot/server/internal/render/artifact"
)

type Result = renderartifact.Result
type Artifact = renderartifact.Artifact

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
	return renderartifact.BuildCacheKey(artifactRequest(request), version, sourceDigest, resourceDigest, deviceScalePercent, payloadBytes)
}

func buildPreviewHTMLCacheKey(request Request, revisionID string, payloadBytes []byte) string {
	return renderartifact.BuildPreviewHTMLCacheKey(artifactRequest(request), revisionID, payloadBytes)
}

func buildArtifactID(cacheKey string) string {
	return renderartifact.BuildArtifactID(cacheKey)
}

func (s *Service) persistArtifact(request Request, cacheKey string, content []byte) (Result, error) {
	result, artifact, err := renderartifact.Persist(s.outputRoot, artifactRequest(request), cacheKey, content)
	if err != nil {
		return Result{}, err
	}

	s.mu.Lock()
	s.artifacts[artifact.ArtifactID] = artifact
	s.mu.Unlock()
	return result, nil
}

func (s *Service) loadArtifacts() error {
	cache, artifacts, err := renderartifact.Load(s.outputRoot)
	if err != nil {
		return err
	}
	for cacheKey, result := range cache {
		s.cache[cacheKey] = result
	}
	for artifactID, artifact := range artifacts {
		s.artifacts[artifactID] = artifact
	}
	return nil
}

func (s *Service) LookupArtifact(artifactID string) (Artifact, error) {
	if s == nil {
		return Artifact{}, &Error{Code: "platform.resource_missing", Message: "render service is not available"}
	}

	s.mu.RLock()
	if artifact, ok := s.artifacts[artifactID]; ok {
		s.mu.RUnlock()
		return artifact, nil
	}
	s.mu.RUnlock()

	artifact, err := renderartifact.Lookup(s.outputRoot, artifactID)
	if err != nil {
		var artifactErr *renderartifact.Error
		if errors.As(err, &artifactErr) {
			return Artifact{}, &Error{Code: artifactErr.Code, Message: artifactErr.Message, Err: artifactErr.Err}
		}
		return Artifact{}, err
	}

	s.mu.Lock()
	s.artifacts[artifactID] = artifact
	s.mu.Unlock()
	return artifact, nil
}

func artifactRequest(request Request) renderartifact.Request {
	return renderartifact.Request{
		Template: request.Template,
		Theme:    request.Theme,
		Output:   request.Output,
	}
}
