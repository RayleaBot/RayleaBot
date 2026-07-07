package service

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

	renderrepo "github.com/RayleaBot/RayleaBot/server/internal/render/repository"
)

func (s *Service) ListTemplates(ctx context.Context) ([]TemplateSummary, error) {
	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return nil, err
	}

	items, err := s.templateRepo.ListTemplateSummaries(ctx)
	if err != nil {
		return nil, fmt.Errorf("list render templates: %w", err)
	}
	summaries := make([]TemplateSummary, 0, len(items))
	for _, item := range items {
		summaries = append(summaries, templateSummaryFromRepo(item))
	}
	return summaries, nil
}

func (s *Service) GetTemplate(ctx context.Context, templateID string) (TemplateDetail, error) {
	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return TemplateDetail{}, err
	}

	detail, err := s.getTemplate(ctx, templateID)
	if err != nil {
		return TemplateDetail{}, err
	}
	return templateDetailFromRepo(detail), nil
}

func (s *Service) getTemplate(ctx context.Context, templateID string) (renderrepo.TemplateDetail, error) {
	detail, err := s.templateRepo.GetTemplateDetail(ctx, strings.TrimSpace(templateID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return renderrepo.TemplateDetail{}, &Error{
				Code:    "platform.template_not_found",
				Message: "render template was not found",
			}
		}
		return renderrepo.TemplateDetail{}, fmt.Errorf("get render template %s: %w", templateID, err)
	}
	return detail, nil
}

func (s *Service) GetTemplateSource(ctx context.Context, templateID string) (string, TemplateSource, error) {
	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return "", TemplateSource{}, err
	}

	revisionID, source, err := s.getTemplateSource(ctx, templateID)
	if err != nil {
		return "", TemplateSource{}, err
	}
	return revisionID, templateSourceFromRepo(source), nil
}

func (s *Service) getTemplateSource(ctx context.Context, templateID string) (string, renderrepo.TemplateSource, error) {
	revisionID, source, err := s.templateRepo.GetCurrentSource(ctx, strings.TrimSpace(templateID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", renderrepo.TemplateSource{}, &Error{
				Code:    "platform.template_not_found",
				Message: "render template was not found",
			}
		}
		return "", renderrepo.TemplateSource{}, fmt.Errorf("get render template source %s: %w", templateID, err)
	}
	return revisionID, source, nil
}

func (s *Service) GetTemplatePreviewData(ctx context.Context, templateID string) (map[string]any, error) {
	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return nil, err
	}
	templateID = strings.TrimSpace(templateID)
	if _, err := s.getTemplate(ctx, templateID); err != nil {
		return nil, err
	}

	return s.readTemplatePreviewData(templateID)
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
		Detail:      templateDetailFromRepo(detail),
		Source:      templateSourceFromRepo(source),
		PreviewData: previewData,
	}, nil
}

func (s *Service) readTemplatePreviewData(templateID string) (map[string]any, error) {
	templateDir := s.templateDirFor(templateID)
	previewPath, err := TemplateFilePath(templateDir, DefaultPreviewData)
	if err != nil {
		return nil, &Error{
			Code:    "platform.resource_missing",
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
			Code:    "platform.template_source_invalid",
			Message: "render template preview data is invalid",
			Err:     err,
		}
	}
	return previewData, nil
}

func (s *Service) ValidateTemplate(ctx context.Context, templateID string, source *TemplateSource) (TemplateValidationResult, error) {
	templateID = strings.TrimSpace(templateID)
	if templateID == "" {
		return TemplateValidationResult{}, &Error{Code: "platform.template_not_found", Message: "render template was not found"}
	}

	if exists, err := s.templateRepo.TemplateExists(ctx, templateID); err != nil {
		return TemplateValidationResult{}, fmt.Errorf("query render template %s: %w", templateID, err)
	} else if !exists {
		return TemplateValidationResult{}, &Error{
			Code:    "platform.template_not_found",
			Message: "render template was not found",
		}
	}

	var sourceValue renderrepo.TemplateSource
	if source == nil {
		_, currentSource, err := s.templateRepo.GetCurrentSource(ctx, templateID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return TemplateValidationResult{}, &Error{
					Code:    "platform.template_not_found",
					Message: "render template was not found",
				}
			}
			return TemplateValidationResult{}, fmt.Errorf("get render template source %s: %w", templateID, err)
		}
		sourceValue = currentSource
	} else {
		sourceValue = templateSourceToRepo(*source)
	}

	bundle, err := BuildSourceBundle(templateID, sourceValue)
	if err != nil {
		_ = s.templateRepo.UpdateValidationStatus(ctx, templateID, newValidationStatus(false, 1))
		return TemplateValidationResult{}, err
	}

	_, issues, err := CompileBundle(bundle)
	if err != nil {
		return TemplateValidationResult{}, fmt.Errorf("validate render template %s: %w", templateID, err)
	}

	status := newValidationStatus(len(issues) == 0, len(issues))
	if err := s.templateRepo.UpdateValidationStatus(ctx, templateID, status); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return TemplateValidationResult{}, fmt.Errorf("update render template validation %s: %w", templateID, err)
	}

	return TemplateValidationResult{
		Valid:              len(issues) == 0,
		Issues:             issuesOrEmpty(issues),
		NormalizedManifest: bundle.NormalizedManifest,
	}, nil
}
func (s *Service) ListTemplateVersions(ctx context.Context, templateID string) ([]TemplateVersion, error) {
	items, err := s.templateRepo.ListTemplateVersions(ctx, strings.TrimSpace(templateID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &Error{
				Code:    "platform.template_not_found",
				Message: "render template was not found",
			}
		}
		return nil, fmt.Errorf("list render template versions %s: %w", templateID, err)
	}
	versions := make([]TemplateVersion, 0, len(items))
	for _, item := range items {
		versions = append(versions, templateVersionFromRepo(item))
	}
	return versions, nil
}

func templateSourceFromRepo(source renderrepo.TemplateSource) TemplateSource {
	return TemplateSource{
		ManifestJSON:    source.ManifestJSON,
		HTML:            source.HTML,
		Stylesheet:      source.Stylesheet,
		InputSchemaJSON: source.InputSchemaJSON,
	}
}

func templateSourceToRepo(source TemplateSource) renderrepo.TemplateSource {
	return renderrepo.TemplateSource{
		ManifestJSON:    source.ManifestJSON,
		HTML:            source.HTML,
		Stylesheet:      source.Stylesheet,
		InputSchemaJSON: source.InputSchemaJSON,
	}
}

func templateFilesFromRepo(files renderrepo.TemplateFiles) TemplateFiles {
	return TemplateFiles{
		Manifest:    files.Manifest,
		HTML:        files.HTML,
		Stylesheet:  files.Stylesheet,
		InputSchema: files.InputSchema,
	}
}

func templateValidationStatusFromRepo(status renderrepo.TemplateValidationStatus) TemplateValidationStatus {
	return TemplateValidationStatus{
		Valid:      status.Valid,
		CheckedAt:  status.CheckedAt,
		IssueCount: status.IssueCount,
	}
}

func templateSourceInfoFromRepo(source renderrepo.TemplateSourceInfo) TemplateSourceInfo {
	return TemplateSourceInfo{
		Type:     source.Type,
		PluginID: source.PluginID,
		LocalID:  source.LocalID,
	}
}

func templateVersionFromRepo(version renderrepo.TemplateVersion) TemplateVersion {
	return TemplateVersion{
		RevisionID:      version.RevisionID,
		TemplateVersion: version.TemplateVersion,
		SavedAt:         version.SavedAt,
		Kind:            version.Kind,
		Message:         version.Message,
	}
}

func templateSummaryFromRepo(item renderrepo.TemplateSummary) TemplateSummary {
	return TemplateSummary{
		ID:                item.ID,
		Version:           item.Version,
		Width:             item.Width,
		Height:            item.Height,
		HasInputSchema:    item.HasInputSchema,
		CurrentRevisionID: item.CurrentRevisionID,
		UpdatedAt:         item.UpdatedAt,
		Source:            templateSourceInfoFromRepo(item.Source),
	}
}

func templateDetailFromRepo(detail renderrepo.TemplateDetail) TemplateDetail {
	return TemplateDetail{
		TemplateSummary: templateSummaryFromRepo(detail.TemplateSummary),
		Files:           templateFilesFromRepo(detail.Files),
		CurrentRevision: templateVersionFromRepo(detail.CurrentRevision),
		LastValidation:  templateValidationStatusFromRepo(detail.LastValidation),
	}
}

func (s *Service) rememberTemplateRoot(templateID, templateDir, resourceRoot string) {
	if s == nil {
		return
	}
	s.templateRoots.Remember(templateID, templateDir, resourceRoot)
}

func (s *Service) templateDirFor(templateID string) string {
	if s == nil {
		return ""
	}
	return s.templateRoots.TemplateDir(templateID)
}

func (s *Service) templateRootFor(templateID string) Root {
	if s == nil {
		return Root{}
	}
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
