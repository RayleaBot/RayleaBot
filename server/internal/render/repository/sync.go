package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

func (r *SQLiteTemplateRepository) SyncTemplateRevision(ctx context.Context, revision StoredTemplateRevision, validation TemplateValidationStatus, sourceInfo TemplateSourceInfo) (bool, error) {
	tx, err := r.write.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin render template sync transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	q := r.writeQ.WithTx(tx)
	state, err := q.GetRenderTemplateSyncState(ctx, revision.TemplateID)
	currentSource := TemplateSourceInfo{Type: state.SourceType}
	if state.SourcePluginID.Valid {
		currentSource.PluginID = state.SourcePluginID.String
	}
	if state.SourceLocalID.Valid {
		currentSource.LocalID = state.SourceLocalID.String
	}
	nextSource := normalizedTemplateSourceInfo(sourceInfo)
	switch {
	case err == nil && state.SourceDigest == revision.SourceDigest:
		if currentSource != nextSource {
			return false, fmt.Errorf("render template %s is already registered by %s source", revision.TemplateID, currentSource.Type)
		}
		if state.ValidationValid != int64(boolToInt(validation.Valid)) || state.ValidationIssueCount != int64(validation.IssueCount) {
			if updateErr := q.UpdateRenderTemplateSyncMetadata(ctx, sqlcgen.UpdateRenderTemplateSyncMetadataParams{
				ValidationValid:      int64(boolToInt(validation.Valid)),
				ValidationCheckedAt:  validation.CheckedAt,
				ValidationIssueCount: int64(validation.IssueCount),
				SourceType:           nextSource.Type,
				SourcePluginID:       nullableString(nextSource.PluginID),
				SourceLocalID:        nullableString(nextSource.LocalID),
				TemplateID:           revision.TemplateID,
			}); updateErr != nil {
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
	if state.CurrentRevisionID == "" {
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
