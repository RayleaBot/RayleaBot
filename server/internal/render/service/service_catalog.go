package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	renderrepo "github.com/RayleaBot/RayleaBot/server/internal/render/repository"
	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
)

func (s *Service) ListTemplates(ctx context.Context) ([]renderrepo.TemplateSummary, error) {
	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return nil, err
	}

	items, err := s.templateRepo.ListTemplateSummaries(ctx)
	if err != nil {
		return nil, fmt.Errorf("list render templates: %w", err)
	}
	return items, nil
}
func (s *Service) GetTemplate(ctx context.Context, templateID string) (renderrepo.TemplateDetail, error) {
	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return renderrepo.TemplateDetail{}, err
	}

	detail, err := s.templateRepo.GetTemplateDetail(ctx, strings.TrimSpace(templateID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return renderrepo.TemplateDetail{}, &rendertemplates.Error{
				Code:    "platform.template_not_found",
				Message: "render template was not found",
			}
		}
		return renderrepo.TemplateDetail{}, fmt.Errorf("get render template %s: %w", templateID, err)
	}
	return detail, nil
}
func (s *Service) GetTemplateSource(ctx context.Context, templateID string) (string, renderrepo.TemplateSource, error) {
	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return "", renderrepo.TemplateSource{}, err
	}

	revisionID, source, err := s.templateRepo.GetCurrentSource(ctx, strings.TrimSpace(templateID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", renderrepo.TemplateSource{}, &rendertemplates.Error{
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
	if _, err := s.GetTemplate(ctx, templateID); err != nil {
		return nil, err
	}

	templateDir := s.templateDirFor(templateID)
	previewPath, err := rendertemplates.TemplateFilePath(templateDir, rendertemplates.DefaultPreviewData)
	if err != nil {
		return nil, &rendertemplates.Error{
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
		return nil, &rendertemplates.Error{
			Code:    "platform.template_source_invalid",
			Message: "render template preview data is invalid",
			Err:     err,
		}
	}
	return previewData, nil
}
func (s *Service) ValidateTemplate(ctx context.Context, templateID string, source *renderrepo.TemplateSource) (rendertemplates.TemplateValidationResult, error) {
	templateID = strings.TrimSpace(templateID)
	if templateID == "" {
		return rendertemplates.TemplateValidationResult{}, &rendertemplates.Error{Code: "platform.template_not_found", Message: "render template was not found"}
	}

	if exists, err := s.templateRepo.TemplateExists(ctx, templateID); err != nil {
		return rendertemplates.TemplateValidationResult{}, fmt.Errorf("query render template %s: %w", templateID, err)
	} else if !exists {
		return rendertemplates.TemplateValidationResult{}, &rendertemplates.Error{
			Code:    "platform.template_not_found",
			Message: "render template was not found",
		}
	}

	var sourceValue renderrepo.TemplateSource
	if source == nil {
		_, currentSource, err := s.templateRepo.GetCurrentSource(ctx, templateID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return rendertemplates.TemplateValidationResult{}, &rendertemplates.Error{
					Code:    "platform.template_not_found",
					Message: "render template was not found",
				}
			}
			return rendertemplates.TemplateValidationResult{}, fmt.Errorf("get render template source %s: %w", templateID, err)
		}
		sourceValue = currentSource
	} else {
		sourceValue = *source
	}

	bundle, err := rendertemplates.BuildSourceBundle(templateID, sourceValue)
	if err != nil {
		_ = s.templateRepo.UpdateValidationStatus(ctx, templateID, newValidationStatus(false, 1))
		return rendertemplates.TemplateValidationResult{}, err
	}

	_, issues, err := rendertemplates.CompileBundle(bundle)
	if err != nil {
		return rendertemplates.TemplateValidationResult{}, fmt.Errorf("validate render template %s: %w", templateID, err)
	}

	status := newValidationStatus(len(issues) == 0, len(issues))
	if err := s.templateRepo.UpdateValidationStatus(ctx, templateID, status); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return rendertemplates.TemplateValidationResult{}, fmt.Errorf("update render template validation %s: %w", templateID, err)
	}

	return rendertemplates.TemplateValidationResult{
		Valid:              len(issues) == 0,
		Issues:             issuesOrEmpty(issues),
		NormalizedManifest: bundle.NormalizedManifest,
	}, nil
}
func (s *Service) ListTemplateVersions(ctx context.Context, templateID string) ([]renderrepo.TemplateVersion, error) {
	items, err := s.templateRepo.ListTemplateVersions(ctx, strings.TrimSpace(templateID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &rendertemplates.Error{
				Code:    "platform.template_not_found",
				Message: "render template was not found",
			}
		}
		return nil, fmt.Errorf("list render template versions %s: %w", templateID, err)
	}
	return items, nil
}
