package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	renderrepo "github.com/RayleaBot/RayleaBot/server/internal/render/repository"
)

const (
	ManifestFilename              = "template.json"
	DefaultPreviewData            = "preview.json"
	defaultTemplateVersion        = "1"
	defaultTemplateHTMLFile       = "template.html"
	defaultTemplateStylesheetFile = "styles.css"
	defaultTemplateInputSchema    = "input.schema.json"
	defaultTemplateWidth          = 960
	defaultTemplateHeight         = 640
)

var templateIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type Root struct {
	TemplateDir  string
	ResourceRoot string
}

type TemplateDraft struct {
	Source renderrepo.TemplateSource `json:"source"`
}

type TemplateValidationIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

type TemplateValidationResult struct {
	Valid              bool                      `json:"valid"`
	Issues             []TemplateValidationIssue `json:"issues"`
	NormalizedManifest map[string]any            `json:"normalized_manifest"`
}

type Manifest struct {
	ID          string
	Version     string
	EntryHTML   string
	Stylesheet  string
	InputSchema *string
	Width       int
	Height      int
}

type SourceBundle struct {
	Manifest           Manifest
	NormalizedManifest map[string]any
	Source             renderrepo.TemplateSource
	Files              renderrepo.TemplateFiles
	Digest             string
}

type CompiledTemplate struct {
	Bundle     SourceBundle
	Stylesheet template.CSS
	Schema     *config.Validator
	HTML       *template.Template
}

type Seed struct {
	Source   renderrepo.TemplateSource
	Compiled *CompiledTemplate
}

func ResolveAssetPath(root Root, relativePath string) (string, error) {
	templateDir := strings.TrimSpace(root.TemplateDir)
	resourceRoot := strings.TrimSpace(root.ResourceRoot)
	relativePath = strings.TrimSpace(relativePath)
	if templateDir == "" || resourceRoot == "" || relativePath == "" || filepath.IsAbs(filepath.FromSlash(relativePath)) {
		return "", &Error{Code: "platform.resource_missing", Message: "render template asset was not found"}
	}

	cleanRelative := filepath.Clean(filepath.FromSlash(relativePath))
	if cleanRelative == "." {
		return "", &Error{Code: "platform.resource_missing", Message: "render template asset was not found"}
	}

	absoluteTemplateDir, err := filepath.Abs(templateDir)
	if err != nil {
		return "", err
	}
	absoluteResourceRoot, err := filepath.Abs(resourceRoot)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(absoluteTemplateDir, cleanRelative)
	if !pathWithinRoot(absoluteResourceRoot, candidate) {
		return "", &Error{Code: "platform.resource_missing", Message: "render template asset was not found"}
	}
	return candidate, nil
}

func ManagedSourcePaths(templateDir string, files renderrepo.TemplateFiles) []string {
	relativePaths := []string{
		ManifestFilename,
		DefaultPreviewData,
	}
	if strings.TrimSpace(files.HTML) != "" {
		relativePaths = append(relativePaths, files.HTML)
	}
	if strings.TrimSpace(files.Stylesheet) != "" {
		relativePaths = append(relativePaths, files.Stylesheet)
	}
	if files.InputSchema != nil && strings.TrimSpace(*files.InputSchema) != "" {
		relativePaths = append(relativePaths, *files.InputSchema)
	}

	paths := make([]string, 0, len(relativePaths))
	seen := map[string]struct{}{}
	for _, relativePath := range relativePaths {
		path, err := TemplateFilePath(templateDir, relativePath)
		if err != nil {
			continue
		}
		absolutePath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		key := normalizedFilePath(absolutePath)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		paths = append(paths, absolutePath)
	}
	return paths
}

func (s *Service) LookupTemplateAsset(ctx context.Context, templateID string, relativePath string) (TemplateAsset, error) {
	if s == nil {
		return TemplateAsset{}, &Error{Code: "platform.resource_missing", Message: "render service is not available"}
	}
	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return TemplateAsset{}, err
	}

	templateID = strings.TrimSpace(templateID)
	relativePath = strings.TrimSpace(relativePath)
	if relativePath == "" {
		return TemplateAsset{}, &Error{Code: "platform.resource_missing", Message: "render template asset was not found"}
	}
	if _, err := s.GetTemplate(ctx, templateID); err != nil {
		return TemplateAsset{}, err
	}

	root := s.templateRootFor(templateID)
	if root.TemplateDir == "" || root.ResourceRoot == "" {
		return TemplateAsset{}, &Error{Code: "platform.resource_missing", Message: "render template asset was not found"}
	}
	assetPath, err := ResolveAssetPath(root, relativePath)
	if err != nil {
		return TemplateAsset{}, err
	}
	isSourcePath, err := IsManagedTemplateSourcePath(ctx, s.templateRepo, s.templateRoots, assetPath)
	if err != nil {
		return TemplateAsset{}, err
	}
	if isSourcePath {
		return TemplateAsset{}, &Error{Code: "platform.resource_missing", Message: "render template asset was not found"}
	}
	info, err := os.Stat(assetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return TemplateAsset{}, &Error{Code: "platform.resource_missing", Message: "render template asset was not found", Err: err}
		}
		return TemplateAsset{}, fmt.Errorf("inspect render template asset %s: %w", assetPath, err)
	}
	if info.IsDir() {
		return TemplateAsset{}, &Error{Code: "platform.resource_missing", Message: "render template asset was not found"}
	}

	return TemplateAsset{Path: assetPath}, nil
}

type TemplateRepository interface {
	ListTemplateSummaries(ctx context.Context) ([]renderrepo.TemplateSummary, error)
	GetTemplateDetail(ctx context.Context, templateID string) (renderrepo.TemplateDetail, error)
}

func IsManagedTemplateSourcePath(ctx context.Context, repository TemplateRepository, roots *Roots, candidate string) (bool, error) {
	absoluteCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return false, err
	}
	items, err := repository.ListTemplateSummaries(ctx)
	if err != nil {
		return false, fmt.Errorf("list render templates for asset lookup: %w", err)
	}

	for _, item := range items {
		detail, err := repository.GetTemplateDetail(ctx, item.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return false, fmt.Errorf("get render template %s for asset lookup: %w", item.ID, err)
		}

		root := roots.TemplateRoot(item.ID)
		if root.TemplateDir == "" {
			continue
		}
		for _, sourcePath := range ManagedSourcePaths(root.TemplateDir, detail.Files) {
			if SameFilePath(absoluteCandidate, sourcePath) {
				return true, nil
			}
		}
	}
	return false, nil
}

func SameFilePath(left, right string) bool {
	return normalizedFilePath(left) == normalizedFilePath(right)
}

func normalizedFilePath(path string) string {
	cleaned := filepath.Clean(path)
	if strings.Contains(cleaned, "\\") {
		return strings.ToLower(cleaned)
	}
	return cleaned
}
