package templates

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func DiscoverSeeds(root string, logger *slog.Logger) (map[string]Seed, error) {
	if root == "" {
		return map[string]Seed{}, nil
	}

	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Seed{}, nil
		}
		return nil, fmt.Errorf("inspect templates root %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("templates root %s is not a directory", root)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read templates root %s: %w", root, err)
	}

	seeds := make(map[string]Seed, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		templateDir := filepath.Join(root, entry.Name())
		seed, err := LoadSeed(templateDir)
		if err != nil {
			if logger != nil {
				logger.Warn(
					"render template skipped",
					"component", "render",
					"template_dir", templateDir,
					"err", err,
				)
			}
			continue
		}
		seeds[seed.Compiled.Bundle.Manifest.ID] = seed
	}

	return seeds, nil
}

func LoadSeed(templateDir string) (Seed, error) {
	manifestPath := filepath.Join(templateDir, ManifestFilename)
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return Seed{}, fmt.Errorf("read render template manifest %s: %w", manifestPath, err)
	}

	var manifestJSON map[string]any
	if err := json.Unmarshal(manifestBytes, &manifestJSON); err != nil {
		return Seed{}, fmt.Errorf("decode render template manifest %s: %w", manifestPath, err)
	}

	manifest, normalizedManifest, err := parseTemplateManifest("", manifestJSON)
	if err != nil {
		return Seed{}, fmt.Errorf("load render template manifest %s: %w", manifestPath, err)
	}

	htmlPath, err := TemplateFilePath(templateDir, manifest.EntryHTML)
	if err != nil {
		return Seed{}, fmt.Errorf("resolve render template html for %s: %w", manifest.ID, err)
	}
	htmlBytes, err := os.ReadFile(htmlPath)
	if err != nil {
		return Seed{}, fmt.Errorf("read render template html for %s: %w", manifest.ID, err)
	}

	stylesheetPath, err := TemplateFilePath(templateDir, manifest.Stylesheet)
	if err != nil {
		return Seed{}, fmt.Errorf("resolve render template stylesheet for %s: %w", manifest.ID, err)
	}
	stylesheetBytes, err := os.ReadFile(stylesheetPath)
	if err != nil {
		return Seed{}, fmt.Errorf("read render template stylesheet for %s: %w", manifest.ID, err)
	}

	var inputSchemaJSON map[string]any
	if manifest.InputSchema != nil {
		inputSchemaPath, err := TemplateFilePath(templateDir, *manifest.InputSchema)
		if err != nil {
			return Seed{}, fmt.Errorf("resolve render input schema for %s: %w", manifest.ID, err)
		}
		inputSchemaBytes, err := os.ReadFile(inputSchemaPath)
		if err != nil {
			return Seed{}, fmt.Errorf("read render input schema for %s: %w", manifest.ID, err)
		}
		if err := json.Unmarshal(inputSchemaBytes, &inputSchemaJSON); err != nil {
			return Seed{}, fmt.Errorf("decode render input schema for %s: %w", manifest.ID, err)
		}
	}

	source := TemplateSource{
		ManifestJSON:    normalizedManifest,
		HTML:            string(htmlBytes),
		Stylesheet:      string(stylesheetBytes),
		InputSchemaJSON: inputSchemaJSON,
	}

	bundle, err := BuildSourceBundle(manifest.ID, source)
	if err != nil {
		return Seed{}, err
	}
	compiled, issues, err := CompileBundle(bundle)
	if err != nil {
		return Seed{}, err
	}
	if len(issues) > 0 {
		return Seed{}, fmt.Errorf("render template %s is invalid: %s", manifest.ID, issues[0].Message)
	}

	return Seed{
		Source:   source,
		Compiled: compiled,
	}, nil
}

func TemplateFilePath(templateDir, relativePath string) (string, error) {
	templateDir = strings.TrimSpace(templateDir)
	relativePath = strings.TrimSpace(relativePath)
	if templateDir == "" || relativePath == "" || filepath.IsAbs(filepath.FromSlash(relativePath)) {
		return "", fmt.Errorf("template file path %q is invalid", relativePath)
	}

	cleanRelative := filepath.Clean(filepath.FromSlash(relativePath))
	if cleanRelative == "." || cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("template file path %q is outside template directory", relativePath)
	}

	absoluteRoot, err := filepath.Abs(templateDir)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(absoluteRoot, cleanRelative)
	if !pathWithinRoot(absoluteRoot, candidate) {
		return "", fmt.Errorf("template file path %q is outside template directory", relativePath)
	}
	return candidate, nil
}
