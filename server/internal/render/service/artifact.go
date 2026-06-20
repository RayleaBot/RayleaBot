package service

import (
	"errors"
	"sync"

	renderartifact "github.com/RayleaBot/RayleaBot/server/internal/render/artifact"
	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
)

// artifactStore owns the rendered-artifact and preview caches together with the
// on-disk output root. It guards its own maps so artifact access never contends
// with the render service's runtime-config lock.
type artifactStore struct {
	outputRoot string

	mu               sync.RWMutex
	cache            map[string]renderartifact.Result
	artifacts        map[string]renderartifact.Artifact
	previewHTMLCache map[string]PreviewHTML
}

func newArtifactStore(outputRoot string) *artifactStore {
	return &artifactStore{
		outputRoot:       outputRoot,
		cache:            map[string]renderartifact.Result{},
		artifacts:        map[string]renderartifact.Artifact{},
		previewHTMLCache: map[string]PreviewHTML{},
	}
}

func (a *artifactStore) cachedResult(cacheKey string) (renderartifact.Result, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result, ok := a.cache[cacheKey]
	return result, ok
}

func (a *artifactStore) cacheResult(cacheKey string, result renderartifact.Result) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cache[cacheKey] = result
}

func (a *artifactStore) cachedPreviewHTML(cacheKey string) (PreviewHTML, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	preview, ok := a.previewHTMLCache[cacheKey]
	return preview, ok
}

func (a *artifactStore) cachePreviewHTML(cacheKey string, preview PreviewHTML) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.previewHTMLCache[cacheKey] = preview
}

func (a *artifactStore) persist(request Request, cacheKey string, content []byte) (renderartifact.Result, error) {
	result, artifact, err := renderartifact.Persist(a.outputRoot, artifactRequest(request), cacheKey, content)
	if err != nil {
		return renderartifact.Result{}, err
	}

	a.mu.Lock()
	a.artifacts[artifact.ArtifactID] = artifact
	a.mu.Unlock()
	return result, nil
}

func (a *artifactStore) load() error {
	cache, artifacts, err := renderartifact.Load(a.outputRoot)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for cacheKey, result := range cache {
		a.cache[cacheKey] = result
	}
	for artifactID, artifact := range artifacts {
		a.artifacts[artifactID] = artifact
	}
	return nil
}

func (a *artifactStore) lookup(artifactID string) (renderartifact.Artifact, error) {
	a.mu.RLock()
	if artifact, ok := a.artifacts[artifactID]; ok {
		a.mu.RUnlock()
		return artifact, nil
	}
	a.mu.RUnlock()

	artifact, err := renderartifact.Lookup(a.outputRoot, artifactID)
	if err != nil {
		var artifactErr *renderartifact.Error
		if errors.As(err, &artifactErr) {
			return renderartifact.Artifact{}, &rendertemplates.Error{Code: artifactErr.Code, Message: artifactErr.Message, Err: artifactErr.Err}
		}
		return renderartifact.Artifact{}, err
	}

	a.mu.Lock()
	a.artifacts[artifactID] = artifact
	a.mu.Unlock()
	return artifact, nil
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

func artifactRequest(request Request) renderartifact.Request {
	return renderartifact.Request{
		Template: request.Template,
		Theme:    request.Theme,
		Output:   request.Output,
	}
}
