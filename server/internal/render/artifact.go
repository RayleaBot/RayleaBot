package render

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/fsguard"
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

// artifactStore owns the rendered-artifact and preview caches together with the
// on-disk output root. It guards its own maps so artifact access never contends
// with the render service's runtime-config lock.
type artifactStore struct {
	outputRoot string

	mu               sync.RWMutex
	cache            map[string]Result
	previewHTMLCache map[string]PreviewHTML
}

func newArtifactStore(outputRoot string) *artifactStore {
	return &artifactStore{
		outputRoot:       outputRoot,
		cache:            map[string]Result{},
		previewHTMLCache: map[string]PreviewHTML{},
	}
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
	return Persist(a.outputRoot, request, cacheKey, content)
}

func (a *artifactStore) load() error {
	cache, err := Load(a.outputRoot)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for cacheKey, result := range cache {
		a.cache[cacheKey] = result
	}
	return nil
}

func BuildCacheKey(request Request, version string, sourceDigest string, resourceDigest string, deviceScalePercent int, payloadBytes []byte) string {
	sum := sha256.Sum256(payloadBytes)
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s:%s:%d:%s", "render-cache-v4-prefetched-resources", request.Template, version, sourceDigest, resourceDigest, renderResourcesDigest(request.Resources), request.Theme, request.Output, normalizeArtifactDeviceScalePercent(deviceScalePercent), hex.EncodeToString(sum[:12]))
}

func BuildPreviewHTMLCacheKey(request Request, sourceDigest string, payloadBytes []byte) string {
	sum := sha256.Sum256(payloadBytes)
	return fmt.Sprintf("preview-html:%s:%s:%s:%s", request.Template, sourceDigest, request.Theme, hex.EncodeToString(sum[:12]))
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

func buildPreviewHTMLCacheKey(request Request, sourceDigest string, payloadBytes []byte) string {
	return BuildPreviewHTMLCacheKey(request, sourceDigest, payloadBytes)
}

func Persist(outputRoot string, request Request, cacheKey string, content []byte) (Result, error) {
	artifactID := BuildArtifactID(cacheKey)
	filename := artifactID + outputSuffix(request.Output)
	artifactPath := filepath.Join(outputRoot, filename)
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
	if err := os.WriteFile(filepath.Join(outputRoot, artifactID+".json"), recordBytes, 0o644); err != nil {
		return Result{}, fmt.Errorf("write render artifact record %s: %w", artifactID, err)
	}

	return Result{
		ArtifactID: artifactID,
		ImagePath:  fileURL(artifactPath),
		MIME:       record.MIME,
		CacheKey:   cacheKey,
		Template:   request.Template,
		Theme:      request.Theme,
		FromCache:  false,
	}, nil
}

func Load(outputRoot string) (map[string]Result, error) {
	entries, err := os.ReadDir(outputRoot)
	if err != nil {
		return nil, fmt.Errorf("read render output root %s: %w", outputRoot, err)
	}

	cache := make(map[string]Result)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		recordPath := filepath.Join(outputRoot, entry.Name())
		recordBytes, err := os.ReadFile(recordPath)
		if err != nil {
			return nil, fmt.Errorf("read render artifact record %s: %w", recordPath, err)
		}

		var record artifactRecord
		if err := json.Unmarshal(recordBytes, &record); err != nil {
			return nil, fmt.Errorf("decode render artifact record %s: %w", recordPath, err)
		}

		artifactPath := filepath.Join(outputRoot, filepath.Base(record.Filename))
		if !fsguard.WithinRoot(outputRoot, artifactPath) {
			continue
		}
		if _, err := os.Stat(artifactPath); err != nil {
			continue
		}

		cache[record.CacheKey] = Result{
			ArtifactID: record.ArtifactID,
			ImagePath:  fileURL(artifactPath),
			MIME:       record.MIME,
			CacheKey:   record.CacheKey,
			Template:   record.Template,
			Theme:      record.Theme,
			FromCache:  true,
		}
	}

	return cache, nil
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
	path = filepath.ToSlash(path)
	// A drive letter is part of the URL path, not its authority. Without
	// the leading slash C:/... becomes file://C:/... and loses its volume.
	if filepath.VolumeName(path) != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}
