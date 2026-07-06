package service

import (
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
)

type Result struct {
	ArtifactID string `json:"artifact_id"`
	ImagePath  string `json:"image_path"`
	MIME       string `json:"mime"`
	CacheKey   string `json:"cache_key"`
	Template   string `json:"template"`
	Theme      string `json:"theme"`
	FromCache  bool   `json:"from_cache"`
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

var artifactIDPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// artifactStore owns the rendered-artifact and preview caches together with the
// on-disk output root. It guards its own maps so artifact access never contends
// with the render service's runtime-config lock.
type artifactStore struct {
	outputRoot string

	mu               sync.RWMutex
	cache            map[string]Result
	artifacts        map[string]Artifact
	previewHTMLCache map[string]PreviewHTML
}

func newArtifactStore(outputRoot string) *artifactStore {
	return &artifactStore{
		outputRoot:       outputRoot,
		cache:            map[string]Result{},
		artifacts:        map[string]Artifact{},
		previewHTMLCache: map[string]PreviewHTML{},
	}
}

func (s *Service) LookupArtifact(artifactID string) (Artifact, error) {
	if s == nil {
		return Artifact{}, &Error{Code: "platform.resource_missing", Message: "render service is not available"}
	}
	return s.artifactStore.lookup(artifactID)
}

func (a *artifactStore) cachedResult(cacheKey string) (Result, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result, ok := a.cache[cacheKey]
	return result, ok
}

func (a *artifactStore) cacheResult(cacheKey string, result Result) {
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

func (a *artifactStore) persist(request Request, cacheKey string, content []byte) (Result, error) {
	result, artifact, err := Persist(a.outputRoot, request, cacheKey, content)
	if err != nil {
		return Result{}, err
	}

	a.mu.Lock()
	a.artifacts[artifact.ArtifactID] = artifact
	a.mu.Unlock()
	return result, nil
}

func (a *artifactStore) load() error {
	cache, artifacts, err := Load(a.outputRoot)
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

func (a *artifactStore) lookup(artifactID string) (Artifact, error) {
	a.mu.RLock()
	if artifact, ok := a.artifacts[artifactID]; ok {
		a.mu.RUnlock()
		return artifact, nil
	}
	a.mu.RUnlock()

	artifact, err := Lookup(a.outputRoot, artifactID)
	if err != nil {
		var artifactErr *Error
		if errors.As(err, &artifactErr) {
			return Artifact{}, &Error{Code: artifactErr.Code, Message: artifactErr.Message, Err: artifactErr.Err}
		}
		return Artifact{}, err
	}

	a.mu.Lock()
	a.artifacts[artifactID] = artifact
	a.mu.Unlock()
	return artifact, nil
}

func BuildCacheKey(request Request, version string, sourceDigest string, resourceDigest string, deviceScalePercent int, payloadBytes []byte) string {
	sum := sha256.Sum256(payloadBytes)
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s:%d:%s", "render-cache-v3-template-sources", request.Template, version, sourceDigest, resourceDigest, request.Theme, request.Output, normalizeArtifactDeviceScalePercent(deviceScalePercent), hex.EncodeToString(sum[:12]))
}

func BuildPreviewHTMLCacheKey(request Request, revisionID string, payloadBytes []byte) string {
	sum := sha256.Sum256(payloadBytes)
	return fmt.Sprintf("preview-html:%s:%s:%s:%s", request.Template, revisionID, request.Theme, hex.EncodeToString(sum[:12]))
}

func BuildArtifactID(cacheKey string) string {
	sum := sha256.Sum256([]byte(cacheKey))
	return "artifact_" + hex.EncodeToString(sum[:12])
}

func normalizeArtifactDeviceScalePercent(percent int) int {
	if percent < 10 {
		return 10
	}
	if percent > 400 {
		return 400
	}
	return percent
}

func buildCacheKey(request Request, version string, sourceDigest string, resourceDigest string, deviceScalePercent int, payloadBytes []byte) string {
	return BuildCacheKey(request, version, sourceDigest, resourceDigest, deviceScalePercent, payloadBytes)
}

func buildPreviewHTMLCacheKey(request Request, revisionID string, payloadBytes []byte) string {
	return BuildPreviewHTMLCacheKey(request, revisionID, payloadBytes)
}

func Persist(outputRoot string, request Request, cacheKey string, content []byte) (Result, Artifact, error) {
	artifactID := BuildArtifactID(cacheKey)
	filename := artifactID + outputSuffix(request.Output)
	artifactPath := filepath.Join(outputRoot, filename)
	if err := os.WriteFile(artifactPath, content, 0o644); err != nil {
		return Result{}, Artifact{}, fmt.Errorf("write render artifact %s: %w", artifactPath, err)
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
		return Result{}, Artifact{}, fmt.Errorf("encode render artifact record %s: %w", artifactID, err)
	}
	if err := os.WriteFile(filepath.Join(outputRoot, artifactID+".json"), recordBytes, 0o644); err != nil {
		return Result{}, Artifact{}, fmt.Errorf("write render artifact record %s: %w", artifactID, err)
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
	artifact := Artifact{
		ArtifactID: artifactID,
		MIME:       record.MIME,
		Path:       artifactPath,
	}

	return result, artifact, nil
}

func Load(outputRoot string) (map[string]Result, map[string]Artifact, error) {
	entries, err := os.ReadDir(outputRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("read render output root %s: %w", outputRoot, err)
	}

	cache := make(map[string]Result)
	artifacts := make(map[string]Artifact)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		recordPath := filepath.Join(outputRoot, entry.Name())
		recordBytes, err := os.ReadFile(recordPath)
		if err != nil {
			return nil, nil, fmt.Errorf("read render artifact record %s: %w", recordPath, err)
		}

		var record artifactRecord
		if err := json.Unmarshal(recordBytes, &record); err != nil {
			return nil, nil, fmt.Errorf("decode render artifact record %s: %w", recordPath, err)
		}

		artifactPath := filepath.Join(outputRoot, filepath.Base(record.Filename))
		if !artifactPathWithinRoot(outputRoot, artifactPath) {
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
		cache[record.CacheKey] = result
		artifacts[record.ArtifactID] = Artifact{
			ArtifactID: record.ArtifactID,
			MIME:       record.MIME,
			Path:       artifactPath,
		}
	}

	return cache, artifacts, nil
}

func Lookup(outputRoot string, artifactID string) (Artifact, error) {
	if !artifactIDPattern.MatchString(strings.TrimSpace(artifactID)) {
		return Artifact{}, &Error{Code: "platform.resource_missing", Message: "render artifact was not found"}
	}

	recordPath := filepath.Join(outputRoot, artifactID+".json")
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

	artifactPath := filepath.Join(outputRoot, filepath.Base(record.Filename))
	if !artifactPathWithinRoot(outputRoot, artifactPath) {
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

	return artifact, nil
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

func artifactPathWithinRoot(root, candidate string) bool {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
