package service

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	renderrepo "github.com/RayleaBot/RayleaBot/server/internal/render/repository"
)

func BuildSourceBundle(expectedTemplateID string, source renderrepo.TemplateSource) (SourceBundle, error) {
	manifest, normalizedManifest, err := parseTemplateManifest(expectedTemplateID, source.ManifestJSON)
	if err != nil {
		return SourceBundle{}, &Error{
			Code:    "platform.template_source_invalid",
			Message: "render template source is invalid",
			Err:     err,
		}
	}

	inputSchemaJSON, err := normalizeOptionalJSONObject(source.InputSchemaJSON, "input_schema_json")
	if err != nil {
		return SourceBundle{}, &Error{
			Code:    "platform.template_source_invalid",
			Message: "render template source is invalid",
			Err:     err,
		}
	}

	if manifest.InputSchema == nil && inputSchemaJSON != nil {
		defaultInputSchema := defaultTemplateInputSchema
		manifest.InputSchema = &defaultInputSchema
		normalizedManifest["input_schema"] = defaultInputSchema
	}
	if manifest.InputSchema != nil && inputSchemaJSON == nil {
		return SourceBundle{}, &Error{
			Code:    "platform.template_source_invalid",
			Message: "render template source is invalid",
			Err:     fmt.Errorf("manifest declares input_schema but input_schema_json is null"),
		}
	}

	normalizedSource := renderrepo.TemplateSource{
		ManifestJSON:    normalizedManifest,
		HTML:            source.HTML,
		Stylesheet:      source.Stylesheet,
		InputSchemaJSON: inputSchemaJSON,
	}

	return SourceBundle{
		Manifest:           manifest,
		NormalizedManifest: normalizedManifest,
		Source:             normalizedSource,
		Files: renderrepo.TemplateFiles{
			Manifest:    ManifestFilename,
			HTML:        manifest.EntryHTML,
			Stylesheet:  manifest.Stylesheet,
			InputSchema: manifest.InputSchema,
		},
		Digest: DigestSource(normalizedSource),
	}, nil
}

func normalizeOptionalJSONObject(raw map[string]any, field string) (map[string]any, error) {
	if raw == nil {
		return nil, nil
	}

	bytes, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%s is not serializable: %w", field, err)
	}

	var normalized map[string]any
	if err := json.Unmarshal(bytes, &normalized); err != nil {
		return nil, fmt.Errorf("%s must be a JSON object: %w", field, err)
	}

	return normalized, nil
}

func DigestSource(source renderrepo.TemplateSource) string {
	payload := struct {
		ManifestJSON    map[string]any `json:"manifest_json"`
		HTML            string         `json:"html"`
		Stylesheet      string         `json:"stylesheet"`
		InputSchemaJSON map[string]any `json:"input_schema_json"`
	}{
		ManifestJSON:    source.ManifestJSON,
		HTML:            source.HTML,
		Stylesheet:      source.Stylesheet,
		InputSchemaJSON: source.InputSchemaJSON,
	}
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

const resourceDigestFullRecheckInterval = time.Second

type resourceDigestEntry struct {
	fingerprint  string
	digest       string
	fullyChecked time.Time
}

type resourceFile struct {
	path     string
	relative string
}

type resourceDigester struct {
	mu              sync.Mutex
	cache           map[string]resourceDigestEntry
	now             func() time.Time
	recheckInterval time.Duration
}

var defaultResourceDigester = newResourceDigester(time.Now, resourceDigestFullRecheckInterval)

func newResourceDigester(now func() time.Time, recheckInterval time.Duration) *resourceDigester {
	if now == nil {
		now = time.Now
	}
	return &resourceDigester{
		cache:           make(map[string]resourceDigestEntry),
		now:             now,
		recheckInterval: recheckInterval,
	}
}

func ResourceDigest(templateDir string) (string, error) {
	return defaultResourceDigester.Digest(templateDir)
}

func InvalidateResourceDigest(templateDir string) {
	defaultResourceDigester.Invalidate(templateDir)
}

func (d *resourceDigester) Invalidate(templateDir string) {
	templateDir, err := filepath.Abs(templateDir)
	if err != nil {
		return
	}
	d.mu.Lock()
	delete(d.cache, templateDir)
	d.mu.Unlock()
}

func (d *resourceDigester) Digest(templateDir string) (string, error) {
	templateDir, err := filepath.Abs(templateDir)
	if err != nil {
		return "", fmt.Errorf("resolve render resource directory: %w", err)
	}
	if templateDir == "" {
		return "", fmt.Errorf("resolve render resource directory: empty path")
	}

	files, fingerprint, exists, err := scanResourceFiles(templateDir)
	if err != nil || !exists {
		return "", err
	}

	now := d.now().UTC()
	d.mu.Lock()
	cached, ok := d.cache[templateDir]
	d.mu.Unlock()
	age := now.Sub(cached.fullyChecked)
	if ok && cached.fingerprint == fingerprint && age >= 0 && age < d.recheckInterval {
		return cached.digest, nil
	}

	digest, err := digestResourceFiles(files)
	if err != nil {
		return "", err
	}
	d.mu.Lock()
	d.cache[templateDir] = resourceDigestEntry{
		fingerprint:  fingerprint,
		digest:       digest,
		fullyChecked: now,
	}
	d.mu.Unlock()
	return digest, nil
}

func scanResourceFiles(templateDir string) ([]resourceFile, string, bool, error) {
	assetsDir := filepath.Join(templateDir, "assets")
	if !pathWithinRoot(templateDir, assetsDir) {
		return nil, "", false, fmt.Errorf("render resource directory escapes template root")
	}
	assetsInfo, err := os.Stat(assetsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", false, nil
	}
	if err != nil {
		return nil, "", false, fmt.Errorf("inspect render resource directory: %w", err)
	}
	if !assetsInfo.IsDir() {
		return nil, "", false, fmt.Errorf("render resource path is not a directory")
	}

	files := make([]resourceFile, 0, 32)
	fingerprint := sha256.New()
	var encodedNumber [8]byte
	walkErr := filepath.WalkDir(assetsDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("render resource symlink is not allowed: %s", path)
		}
		relative, err := filepath.Rel(templateDir, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		files = append(files, resourceFile{path: path, relative: relative})
		_, _ = fingerprint.Write([]byte(relative))
		_, _ = fingerprint.Write([]byte{0})
		binary.LittleEndian.PutUint64(encodedNumber[:], uint64(info.Size()))
		_, _ = fingerprint.Write(encodedNumber[:])
		binary.LittleEndian.PutUint64(encodedNumber[:], uint64(info.ModTime().UnixNano()))
		_, _ = fingerprint.Write(encodedNumber[:])
		return nil
	})
	if walkErr != nil {
		return nil, "", false, fmt.Errorf("inspect render resources: %w", walkErr)
	}
	return files, hex.EncodeToString(fingerprint.Sum(nil)), true, nil
}

func digestResourceFiles(files []resourceFile) (string, error) {
	digest := sha256.New()
	buffer := make([]byte, 32*1024)
	for _, file := range files {
		_, _ = digest.Write([]byte(file.relative))
		_, _ = digest.Write([]byte{0})
		reader, err := os.Open(file.path)
		if err != nil {
			return "", fmt.Errorf("open render resource %s: %w", file.relative, err)
		}
		_, copyErr := io.CopyBuffer(digest, reader, buffer)
		closeErr := reader.Close()
		if copyErr != nil {
			return "", fmt.Errorf("digest render resource %s: %w", file.relative, copyErr)
		}
		if closeErr != nil {
			return "", fmt.Errorf("close render resource %s: %w", file.relative, closeErr)
		}
		_, _ = digest.Write([]byte{0})
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
