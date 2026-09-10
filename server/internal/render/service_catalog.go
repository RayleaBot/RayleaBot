package render

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
)

func (s *Service) ListTemplates(ctx context.Context) ([]TemplateSummary, error) {
	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return nil, err
	}

	items, err := s.templateRepo.ListTemplateSummaries(ctx)
	if err != nil {
		return nil, fmt.Errorf("list render templates: %w", err)
	}
	return items, nil
}

func (s *Service) GetTemplate(ctx context.Context, templateID string) (TemplateDetail, error) {
	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return TemplateDetail{}, err
	}

	detail, err := s.getTemplate(ctx, templateID)
	if err != nil {
		return TemplateDetail{}, err
	}
	return detail, nil
}

func (s *Service) getTemplate(ctx context.Context, templateID string) (TemplateDetail, error) {
	detail, err := s.templateRepo.GetTemplateDetail(ctx, strings.TrimSpace(templateID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TemplateDetail{}, &Error{
				Code:    errorcodes.PlatformTemplateNotFound,
				Message: "render template was not found",
			}
		}
		return TemplateDetail{}, fmt.Errorf("get render template %s: %w", templateID, err)
	}
	return detail, nil
}

func (s *Service) GetTemplateSource(ctx context.Context, templateID string) (string, TemplateSource, error) {
	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return "", TemplateSource{}, err
	}

	sourceDigest, source, err := s.getTemplateSource(ctx, templateID)
	if err != nil {
		return "", TemplateSource{}, err
	}
	return sourceDigest, source, nil
}

func (s *Service) getTemplateSource(ctx context.Context, templateID string) (string, TemplateSource, error) {
	sourceDigest, source, err := s.templateRepo.GetCurrentSource(ctx, strings.TrimSpace(templateID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", TemplateSource{}, &Error{
				Code:    errorcodes.PlatformTemplateNotFound,
				Message: "render template was not found",
			}
		}
		return "", TemplateSource{}, fmt.Errorf("get render template source %s: %w", templateID, err)
	}
	return sourceDigest, source, nil
}

func (s *Service) GetTemplateDetailSnapshot(ctx context.Context, templateID string) (TemplateDetailSnapshot, error) {
	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return TemplateDetailSnapshot{}, err
	}
	templateID = strings.TrimSpace(templateID)
	detail, err := s.getTemplate(ctx, templateID)
	if err != nil {
		return TemplateDetailSnapshot{}, err
	}
	_, source, err := s.getTemplateSource(ctx, templateID)
	if err != nil {
		return TemplateDetailSnapshot{}, err
	}
	previewData, err := s.readTemplatePreviewData(templateID)
	if err != nil {
		return TemplateDetailSnapshot{}, err
	}
	return TemplateDetailSnapshot{
		Detail:      detail,
		Source:      source,
		PreviewData: previewData,
	}, nil
}

func (s *Service) readTemplatePreviewData(templateID string) (map[string]any, error) {
	templateDir := s.templateDirFor(templateID)
	previewPath, err := TemplateFilePath(templateDir, DefaultPreviewData)
	if err != nil {
		return nil, &Error{
			Code:    errorcodes.PlatformResourceMissing,
			Message: "render template preview data was not found",
			Err:     err,
		}
	}
	content, err := os.ReadFile(previewPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read render template preview data %s: %w", previewPath, err)
	}

	var previewData map[string]any
	if err := json.Unmarshal(content, &previewData); err != nil {
		return nil, &Error{
			Code:    errorcodes.DiagnosticPlatformTemplateSourceInvalid,
			Message: "render template preview data is invalid",
			Err:     err,
		}
	}
	return previewData, nil
}

func (s *Service) rememberTemplateRoot(templateID, templateDir, resourceRoot string) {
	s.templateRoots.Remember(templateID, templateDir, resourceRoot)
}

func (s *Service) templateDirFor(templateID string) string {
	return s.templateRoots.TemplateDir(templateID)
}

func (s *Service) templateRootFor(templateID string) Root {
	return s.templateRoots.TemplateRoot(templateID)
}

type Roots struct {
	mu            sync.RWMutex
	templatesRoot string
	entries       map[string]Root
}

func NewRoots(templatesRoot string) *Roots {
	return &Roots{
		templatesRoot: templatesRoot,
		entries:       map[string]Root{},
	}
}

func (r *Roots) Remember(templateID, templateDir, resourceRoot string) {
	if r == nil || strings.TrimSpace(templateID) == "" || strings.TrimSpace(templateDir) == "" {
		return
	}
	absoluteTemplateDir, err := filepath.Abs(templateDir)
	if err != nil {
		return
	}
	if strings.TrimSpace(resourceRoot) == "" {
		resourceRoot = templateDir
	}
	absoluteResourceRoot, err := filepath.Abs(resourceRoot)
	if err != nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[strings.TrimSpace(templateID)] = Root{
		TemplateDir:  absoluteTemplateDir,
		ResourceRoot: absoluteResourceRoot,
	}
}

func (r *Roots) TemplateDir(templateID string) string {
	templateID = strings.TrimSpace(templateID)
	if r == nil {
		return ""
	}
	r.mu.RLock()
	if root := r.entries[templateID]; root.TemplateDir != "" {
		r.mu.RUnlock()
		return root.TemplateDir
	}
	r.mu.RUnlock()
	templateDir, ok := templateDirWithinRoot(r.templatesRoot, templateID)
	if !ok {
		return ""
	}
	return templateDir
}

func (r *Roots) TemplateRoot(templateID string) Root {
	templateID = strings.TrimSpace(templateID)
	if r == nil {
		return Root{}
	}
	r.mu.RLock()
	root := r.entries[templateID]
	r.mu.RUnlock()
	if root.TemplateDir != "" && root.ResourceRoot != "" {
		return root
	}
	templateDir, ok := templateDirWithinRoot(r.templatesRoot, templateID)
	if !ok {
		return Root{}
	}
	return Root{
		TemplateDir:  templateDir,
		ResourceRoot: r.templatesRoot,
	}
}

func (r *Roots) RemovePrefix(prefix string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for templateID := range r.entries {
		if strings.HasPrefix(templateID, prefix) {
			delete(r.entries, templateID)
		}
	}
}

func BaseURL(templateDir string) string {
	templateDir, err := filepath.Abs(templateDir)
	if err != nil || templateDir == "" {
		return ""
	}
	path := filepath.ToSlash(templateDir)
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return (&url.URL{
		Scheme: "file",
		Path:   path,
	}).String()
}

func templateDirWithinRoot(root string, templateID string) (string, bool) {
	root = strings.TrimSpace(root)
	templateID = strings.TrimSpace(templateID)
	if root == "" || templateID == "" || filepath.IsAbs(filepath.FromSlash(templateID)) {
		return "", false
	}
	cleanID := filepath.Clean(filepath.FromSlash(templateID))
	if cleanID == "." || cleanID == ".." || strings.HasPrefix(cleanID, ".."+string(filepath.Separator)) {
		return "", false
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	candidate := filepath.Join(absoluteRoot, cleanID)
	if !pathWithinRoot(absoluteRoot, candidate) {
		return "", false
	}
	return candidate, true
}

func pathWithinRoot(root, candidate string) bool {
	relativePath, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relativePath != ".." && !strings.HasPrefix(relativePath, ".."+string(filepath.Separator))
}
