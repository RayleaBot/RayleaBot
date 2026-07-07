package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (r *SQLiteTemplateRepository) SyncTemplateRevision(ctx context.Context, revision StoredTemplateRevision, validation TemplateValidationStatus, sourceInfo TemplateSourceInfo) (bool, error) {
	tx, err := r.write.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin render template sync transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var (
		currentRevisionID string
		currentDigest     string
		validationValid   bool
		validationIssues  int
		currentSource     TemplateSourceInfo
		sourcePluginID    sql.NullString
		sourceLocalID     sql.NullString
	)
	err = tx.QueryRowContext(ctx, `
		SELECT s.current_revision_id, r.source_digest, s.validation_valid, s.validation_issue_count, s.source_type, s.source_plugin_id, s.source_local_id
		FROM render_template_states s
		INNER JOIN render_template_revisions r ON r.revision_id = s.current_revision_id
		WHERE s.template_id = ?`, revision.TemplateID).Scan(
		&currentRevisionID,
		&currentDigest,
		&validationValid,
		&validationIssues,
		&currentSource.Type,
		&sourcePluginID,
		&sourceLocalID,
	)
	if sourcePluginID.Valid {
		currentSource.PluginID = sourcePluginID.String
	}
	if sourceLocalID.Valid {
		currentSource.LocalID = sourceLocalID.String
	}
	nextSource := normalizedTemplateSourceInfo(sourceInfo)
	switch {
	case err == nil && currentDigest == revision.SourceDigest:
		if currentSource != nextSource {
			return false, fmt.Errorf("render template %s is already registered by %s source", revision.TemplateID, currentSource.Type)
		}
		if validationValid != validation.Valid || validationIssues != validation.IssueCount {
			if _, updateErr := tx.ExecContext(ctx, `
				UPDATE render_template_states
				SET validation_valid = ?, validation_checked_at = ?, validation_issue_count = ?, source_type = ?, source_plugin_id = ?, source_local_id = ?
				WHERE template_id = ?`,
				boolToInt(validation.Valid),
				validation.CheckedAt,
				validation.IssueCount,
				nextSource.Type,
				nullableString(nextSource.PluginID),
				nullableString(nextSource.LocalID),
				revision.TemplateID,
			); updateErr != nil {
				return false, fmt.Errorf("update render template validation during sync for %s: %w", revision.TemplateID, updateErr)
			}
		}
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("commit render template sync transaction: %w", err)
		}
		return false, nil
	case err == nil:
		if currentSource != nextSource {
			return false, fmt.Errorf("render template %s is already registered by %s source", revision.TemplateID, currentSource.Type)
		}
	case errors.Is(err, sql.ErrNoRows):
	default:
		return false, fmt.Errorf("query render template state for sync %s: %w", revision.TemplateID, err)
	}

	if err := insertTemplateRevision(ctx, tx, revision); err != nil {
		return false, err
	}
	if currentRevisionID == "" {
		if err := insertTemplateState(ctx, tx, StoredTemplateState{
			TemplateID:           revision.TemplateID,
			CurrentRevisionID:    revision.RevisionID,
			UpdatedAt:            revision.SavedAt,
			ValidationValid:      validation.Valid,
			ValidationCheckedAt:  validation.CheckedAt,
			ValidationIssueCount: validation.IssueCount,
			Source:               nextSource,
		}); err != nil {
			return false, err
		}
	} else if err := upsertTemplateState(ctx, tx, StoredTemplateState{
		TemplateID:           revision.TemplateID,
		CurrentRevisionID:    revision.RevisionID,
		UpdatedAt:            revision.SavedAt,
		ValidationValid:      validation.Valid,
		ValidationCheckedAt:  validation.CheckedAt,
		ValidationIssueCount: validation.IssueCount,
		Source:               nextSource,
	}); err != nil {
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit render template sync transaction: %w", err)
	}
	return true, nil
}
