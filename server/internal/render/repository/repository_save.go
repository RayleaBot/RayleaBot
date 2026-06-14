package repository

import (
	"context"
	"fmt"
)

func (r *SQLiteTemplateRepository) SaveCurrentRevision(
	ctx context.Context,
	templateID string,
	baseRevisionID string,
	revision StoredTemplateRevision,
	validation TemplateValidationStatus,
) error {
	tx, err := r.write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin render template save transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	state, err := loadTemplateStateTx(ctx, tx, templateID)
	if err != nil {
		return err
	}
	if state.CurrentRevisionID != baseRevisionID {
		return &Error{
			Code:    "platform.template_revision_conflict",
			Message: "render template revision is stale",
		}
	}

	if err := insertTemplateRevision(ctx, tx, revision); err != nil {
		return err
	}
	if err := upsertTemplateState(ctx, tx, StoredTemplateState{
		TemplateID:           templateID,
		CurrentRevisionID:    revision.RevisionID,
		UpdatedAt:            revision.SavedAt,
		ValidationValid:      validation.Valid,
		ValidationCheckedAt:  validation.CheckedAt,
		ValidationIssueCount: validation.IssueCount,
		Source:               state.Source,
	}); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit render template save transaction: %w", err)
	}
	return nil
}
