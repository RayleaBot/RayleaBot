package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	renderrepo "github.com/RayleaBot/RayleaBot/server/internal/render/repository"
)

func (s *Service) UpdateTemplateSource(ctx context.Context, templateID, baseRevisionID, message string, source TemplateSource) (TemplateDetail, error) {
	templateID = strings.TrimSpace(templateID)
	baseRevisionID = strings.TrimSpace(baseRevisionID)
	message = strings.TrimSpace(message)

	bundle, compiled, validation, err := s.validateTemplateForWrite(ctx, templateID, templateSourceToRepo(source))
	if err != nil {
		return TemplateDetail{}, err
	}

	savedAt := time.Now().UTC().Format(time.RFC3339Nano)
	revision := newStoredRevision(templateID, newRevisionID(templateID, bundle.Digest), compiled, "save", &message, savedAt)
	if err := s.templateRepo.SaveCurrentRevision(ctx, templateID, baseRevisionID, revision, validation); err != nil {
		return TemplateDetail{}, s.mapTemplateWriteError(err)
	}

	return s.GetTemplate(ctx, templateID)
}

func (s *Service) RollbackTemplate(ctx context.Context, templateID, targetRevisionID, baseRevisionID, message string) (TemplateDetail, error) {
	templateID = strings.TrimSpace(templateID)
	targetRevisionID = strings.TrimSpace(targetRevisionID)
	baseRevisionID = strings.TrimSpace(baseRevisionID)
	message = strings.TrimSpace(message)

	state, _, err := s.templateRepo.LoadCurrentTemplate(ctx, templateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TemplateDetail{}, &Error{
				Code:    "platform.template_not_found",
				Message: "render template was not found",
			}
		}
		return TemplateDetail{}, fmt.Errorf("get render template state %s: %w", templateID, err)
	}
	if state.CurrentRevisionID != baseRevisionID {
		return TemplateDetail{}, &Error{
			Code:    "platform.template_revision_conflict",
			Message: "render template revision is stale",
		}
	}
	if targetRevisionID == state.CurrentRevisionID {
		return TemplateDetail{}, &Error{
			Code:    "platform.template_rollback_target_invalid",
			Message: "render template rollback target is invalid",
		}
	}

	targetSource, err := s.templateRepo.GetRevisionSource(ctx, templateID, targetRevisionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TemplateDetail{}, &Error{
				Code:    "platform.template_revision_not_found",
				Message: "render template revision was not found",
			}
		}
		return TemplateDetail{}, fmt.Errorf("get render template rollback source %s/%s: %w", templateID, targetRevisionID, err)
	}

	bundle, compiled, validation, err := s.validateTemplateForWrite(ctx, templateID, targetSource)
	if err != nil {
		var renderErr *Error
		if errors.As(err, &renderErr) && renderErr.Code == "platform.template_source_invalid" {
			return TemplateDetail{}, &Error{
				Code:    "platform.template_rollback_target_invalid",
				Message: "render template rollback target is invalid",
			}
		}
		return TemplateDetail{}, err
	}

	savedAt := time.Now().UTC().Format(time.RFC3339Nano)
	revision := newStoredRevision(templateID, newRevisionID(templateID, bundle.Digest), compiled, "rollback", &message, savedAt)
	if err := s.templateRepo.SaveCurrentRevision(ctx, templateID, baseRevisionID, revision, validation); err != nil {
		return TemplateDetail{}, s.mapTemplateWriteError(err)
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

func newStoredRevision(templateID, revisionID string, compiled *CompiledTemplate, kind string, message *string, savedAt string) renderrepo.StoredTemplateRevision {
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

func issuesOrEmpty(issues []TemplateValidationIssue) []TemplateValidationIssue {
	if len(issues) == 0 {
		return []TemplateValidationIssue{}
	}
	return issues
}

func (s *Service) validateTemplateForWrite(ctx context.Context, templateID string, source renderrepo.TemplateSource) (SourceBundle, *CompiledTemplate, renderrepo.TemplateValidationStatus, error) {
	if exists, err := s.templateRepo.TemplateExists(ctx, templateID); err != nil {
		return SourceBundle{}, nil, renderrepo.TemplateValidationStatus{}, fmt.Errorf("query render template %s: %w", templateID, err)
	} else if !exists {
		return SourceBundle{}, nil, renderrepo.TemplateValidationStatus{}, &Error{
			Code:    "platform.template_not_found",
			Message: "render template was not found",
		}
	}

	bundle, err := BuildSourceBundle(templateID, source)
	if err != nil {
		_ = s.templateRepo.UpdateValidationStatus(ctx, templateID, newValidationStatus(false, 1))
		return SourceBundle{}, nil, renderrepo.TemplateValidationStatus{}, err
	}

	compiled, issues, err := CompileBundle(bundle)
	if err != nil {
		return SourceBundle{}, nil, renderrepo.TemplateValidationStatus{}, fmt.Errorf("compile render template %s: %w", templateID, err)
	}

	validation := newValidationStatus(len(issues) == 0, len(issues))
	if err := s.templateRepo.UpdateValidationStatus(ctx, templateID, validation); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return SourceBundle{}, nil, renderrepo.TemplateValidationStatus{}, fmt.Errorf("update render template validation %s: %w", templateID, err)
	}
	if len(issues) > 0 {
		return SourceBundle{}, nil, renderrepo.TemplateValidationStatus{}, &Error{
			Code:    "platform.template_source_invalid",
			Message: issues[0].Message,
		}
	}

	return bundle, compiled, validation, nil
}

func (s *Service) mapTemplateWriteError(err error) error {
	var renderErr *Error
	if errors.As(err, &renderErr) {
		return renderErr
	}
	if errors.Is(err, sql.ErrNoRows) {
		return &Error{
			Code:    "platform.template_not_found",
			Message: "render template was not found",
		}
	}
	return fmt.Errorf("write render template revision: %w", err)
}

func (s *Service) syncTemplatesFromFiles(ctx context.Context) error {
	if s == nil {
		return nil
	}

	s.templateSyncMu.Lock()
	defer s.templateSyncMu.Unlock()

	Seeds, err := DiscoverSeeds(s.repoRoot, s.templatesRoot, s.logger)
	if err != nil {
		return err
	}

	for _, templateID := range SortedIDs(Seeds) {
		seed := Seeds[templateID]
		templateDir := filepath.Join(s.templatesRoot, filepath.Clean(templateID))
		if err := s.syncTemplateSeed(ctx, templateID, seed, renderrepo.TemplateSourceInfo{Type: "system"}, templateDir, s.templatesRoot); err != nil {
			return fmt.Errorf("sync render template %s: %w", templateID, err)
		}
	}
	return nil
}

func (s *Service) syncTemplateSeed(ctx context.Context, templateID string, seed Seed, sourceInfo renderrepo.TemplateSourceInfo, templateDir string, resourceRoot string) error {
	savedAt := time.Now().UTC().Format(time.RFC3339Nano)
	revision := newStoredRevision(
		templateID,
		newRevisionID(templateID, seed.Compiled.Bundle.Digest),
		seed.Compiled,
		"save",
		nil,
		savedAt,
	)
	changed, err := s.templateRepo.SyncTemplateRevision(ctx, revision, renderrepo.TemplateValidationStatus{
		Valid:      true,
		CheckedAt:  savedAt,
		IssueCount: 0,
	}, sourceInfo)
	if err != nil {
		return err
	}

	s.rememberTemplateRoot(templateID, templateDir, resourceRoot)
	if changed && s.logger != nil {
		s.logger.Info(
			"渲染模板已同步："+templateID+"（版本 "+revision.RevisionID+"）",
			"component", "render",
			"template_id", templateID,
			"revision_id", revision.RevisionID,
			"source_digest", revision.SourceDigest,
		)
	}
	return nil
}
