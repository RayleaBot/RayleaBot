package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	detail, err := s.templateRepo.GetTemplateDetail(ctx, strings.TrimSpace(templateID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TemplateDetail{}, &Error{
				Code:    "platform.template_not_found",
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

	revisionID, source, err := s.templateRepo.GetCurrentSource(ctx, strings.TrimSpace(templateID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", TemplateSource{}, &Error{
				Code:    "platform.template_not_found",
				Message: "render template was not found",
			}
		}
		return "", TemplateSource{}, fmt.Errorf("get render template source %s: %w", templateID, err)
	}
	return revisionID, source, nil
}
func (s *Service) GetTemplatePreviewData(ctx context.Context, templateID string) (map[string]any, error) {
	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return nil, err
	}
	templateID = strings.TrimSpace(templateID)
	if _, err := s.GetTemplate(ctx, templateID); err != nil {
		return nil, err
	}

	templateDir := s.templateDirFor(templateID)
	previewPath := filepath.Join(templateDir, defaultTemplatePreviewData)
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

	var sourceValue TemplateSource
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
		sourceValue = *source
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
	return items, nil
}
