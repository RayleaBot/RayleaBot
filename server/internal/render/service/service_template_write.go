package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	renderrepo "github.com/RayleaBot/RayleaBot/server/internal/render/repository"
	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
)

func (s *Service) UpdateTemplateSource(ctx context.Context, templateID, baseRevisionID, message string, source renderrepo.TemplateSource) (renderrepo.TemplateDetail, error) {
	templateID = strings.TrimSpace(templateID)
	baseRevisionID = strings.TrimSpace(baseRevisionID)
	message = strings.TrimSpace(message)

	bundle, compiled, validation, err := s.validateTemplateForWrite(ctx, templateID, source)
	if err != nil {
		return renderrepo.TemplateDetail{}, err
	}

	savedAt := time.Now().UTC().Format(time.RFC3339Nano)
	revision := newStoredRevision(templateID, newRevisionID(templateID, bundle.Digest), compiled, "save", &message, savedAt)
	if err := s.templateRepo.SaveCurrentRevision(ctx, templateID, baseRevisionID, revision, validation); err != nil {
		return renderrepo.TemplateDetail{}, s.mapTemplateWriteError(err)
	}

	return s.GetTemplate(ctx, templateID)
}

func (s *Service) RollbackTemplate(ctx context.Context, templateID, targetRevisionID, baseRevisionID, message string) (renderrepo.TemplateDetail, error) {
	templateID = strings.TrimSpace(templateID)
	targetRevisionID = strings.TrimSpace(targetRevisionID)
	baseRevisionID = strings.TrimSpace(baseRevisionID)
	message = strings.TrimSpace(message)

	state, _, err := s.templateRepo.LoadCurrentTemplate(ctx, templateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return renderrepo.TemplateDetail{}, &rendertemplates.Error{
				Code:    "platform.template_not_found",
				Message: "render template was not found",
			}
		}
		return renderrepo.TemplateDetail{}, fmt.Errorf("get render template state %s: %w", templateID, err)
	}
	if state.CurrentRevisionID != baseRevisionID {
		return renderrepo.TemplateDetail{}, &rendertemplates.Error{
			Code:    "platform.template_revision_conflict",
			Message: "render template revision is stale",
		}
	}
	if targetRevisionID == state.CurrentRevisionID {
		return renderrepo.TemplateDetail{}, &rendertemplates.Error{
			Code:    "platform.template_rollback_target_invalid",
			Message: "render template rollback target is invalid",
		}
	}

	targetSource, err := s.templateRepo.GetRevisionSource(ctx, templateID, targetRevisionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return renderrepo.TemplateDetail{}, &rendertemplates.Error{
				Code:    "platform.template_revision_not_found",
				Message: "render template revision was not found",
			}
		}
		return renderrepo.TemplateDetail{}, fmt.Errorf("get render template rollback source %s/%s: %w", templateID, targetRevisionID, err)
	}

	bundle, compiled, validation, err := s.validateTemplateForWrite(ctx, templateID, targetSource)
	if err != nil {
		var renderErr *rendertemplates.Error
		if errors.As(err, &renderErr) && renderErr.Code == "platform.template_source_invalid" {
			return renderrepo.TemplateDetail{}, &rendertemplates.Error{
				Code:    "platform.template_rollback_target_invalid",
				Message: "render template rollback target is invalid",
			}
		}
		return renderrepo.TemplateDetail{}, err
	}

	savedAt := time.Now().UTC().Format(time.RFC3339Nano)
	revision := newStoredRevision(templateID, newRevisionID(templateID, bundle.Digest), compiled, "rollback", &message, savedAt)
	if err := s.templateRepo.SaveCurrentRevision(ctx, templateID, baseRevisionID, revision, validation); err != nil {
		return renderrepo.TemplateDetail{}, s.mapTemplateWriteError(err)
	}

	return s.GetTemplate(ctx, templateID)
}

func newRevisionID(templateID, digest string) string {
	templateID = strings.NewReplacer(".", "_", "-", "_", "/", "_").Replace(strings.TrimSpace(templateID))
	if len(digest) > 8 {
		digest = digest[:8]
	}
	sequence := atomic.AddUint64(&revisionCounter, 1)
	return fmt.Sprintf("rev_%s_%s_%s_%06d", templateID, time.Now().UTC().Format("20060102T150405000000000"), digest, sequence)
}

func newStoredRevision(templateID, revisionID string, compiled *rendertemplates.CompiledTemplate, kind string, message *string, savedAt string) renderrepo.StoredTemplateRevision {
	manifestJSON, _ := json.Marshal(compiled.Bundle.NormalizedManifest)
	inputSchemaJSON := sql.NullString{}
	if compiled.Bundle.Source.InputSchemaJSON != nil {
		encoded, _ := json.Marshal(compiled.Bundle.Source.InputSchemaJSON)
		inputSchemaJSON = sql.NullString{String: string(encoded), Valid: true}
	}

	return renderrepo.StoredTemplateRevision{
		RevisionID:      revisionID,
		TemplateID:      templateID,
		TemplateVersion: compiled.Bundle.Manifest.Version,
		Kind:            kind,
		Message:         message,
		SavedAt:         savedAt,
		SourceDigest:    compiled.Bundle.Digest,
		ManifestJSON:    string(manifestJSON),
		HTML:            compiled.Bundle.Source.HTML,
		Stylesheet:      compiled.Bundle.Source.Stylesheet,
		InputSchemaJSON: inputSchemaJSON,
	}
}

func newValidationStatus(valid bool, issueCount int) renderrepo.TemplateValidationStatus {
	return renderrepo.TemplateValidationStatus{
		Valid:      valid,
		CheckedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		IssueCount: issueCount,
	}
}

func issuesOrEmpty(issues []rendertemplates.TemplateValidationIssue) []rendertemplates.TemplateValidationIssue {
	if len(issues) == 0 {
		return []rendertemplates.TemplateValidationIssue{}
	}
	return issues
}

func (s *Service) validateTemplateForWrite(ctx context.Context, templateID string, source renderrepo.TemplateSource) (rendertemplates.SourceBundle, *rendertemplates.CompiledTemplate, renderrepo.TemplateValidationStatus, error) {
	if exists, err := s.templateRepo.TemplateExists(ctx, templateID); err != nil {
		return rendertemplates.SourceBundle{}, nil, renderrepo.TemplateValidationStatus{}, fmt.Errorf("query render template %s: %w", templateID, err)
	} else if !exists {
		return rendertemplates.SourceBundle{}, nil, renderrepo.TemplateValidationStatus{}, &rendertemplates.Error{
			Code:    "platform.template_not_found",
			Message: "render template was not found",
		}
	}

	bundle, err := rendertemplates.BuildSourceBundle(templateID, source)
	if err != nil {
		_ = s.templateRepo.UpdateValidationStatus(ctx, templateID, newValidationStatus(false, 1))
		return rendertemplates.SourceBundle{}, nil, renderrepo.TemplateValidationStatus{}, err
	}

	compiled, issues, err := rendertemplates.CompileBundle(bundle)
	if err != nil {
		return rendertemplates.SourceBundle{}, nil, renderrepo.TemplateValidationStatus{}, fmt.Errorf("compile render template %s: %w", templateID, err)
	}

	validation := newValidationStatus(len(issues) == 0, len(issues))
	if err := s.templateRepo.UpdateValidationStatus(ctx, templateID, validation); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return rendertemplates.SourceBundle{}, nil, renderrepo.TemplateValidationStatus{}, fmt.Errorf("update render template validation %s: %w", templateID, err)
	}
	if len(issues) > 0 {
		return rendertemplates.SourceBundle{}, nil, renderrepo.TemplateValidationStatus{}, &rendertemplates.Error{
			Code:    "platform.template_source_invalid",
			Message: issues[0].Message,
		}
	}

	return bundle, compiled, validation, nil
}

func (s *Service) mapTemplateWriteError(err error) error {
	var renderErr *rendertemplates.Error
	if errors.As(err, &renderErr) {
		return renderErr
	}
	if errors.Is(err, sql.ErrNoRows) {
		return &rendertemplates.Error{
			Code:    "platform.template_not_found",
			Message: "render template was not found",
		}
	}
	return fmt.Errorf("write render template revision: %w", err)
}
