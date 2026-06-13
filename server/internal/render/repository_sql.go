package render

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func insertTemplateRevision(ctx context.Context, tx *sql.Tx, revision storedTemplateRevision) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO render_template_revisions (
			revision_id,
			template_id,
			template_version,
			kind,
			message,
			saved_at,
			source_digest,
			manifest_json,
			html,
			stylesheet,
			input_schema_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		revision.RevisionID,
		revision.TemplateID,
		revision.TemplateVersion,
		revision.Kind,
		pointerStringValue(revision.Message),
		revision.SavedAt,
		revision.SourceDigest,
		revision.ManifestJSON,
		revision.HTML,
		revision.Stylesheet,
		revision.InputSchemaJSON,
	)
	if err != nil {
		return fmt.Errorf("insert render template revision %s/%s: %w", revision.TemplateID, revision.RevisionID, err)
	}
	return nil
}

func insertTemplateState(ctx context.Context, tx *sql.Tx, state storedTemplateState) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO render_template_states (
			template_id,
			current_revision_id,
			updated_at,
			validation_valid,
			validation_checked_at,
			validation_issue_count,
			source_type,
			source_plugin_id,
			source_local_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		state.TemplateID,
		state.CurrentRevisionID,
		state.UpdatedAt,
		boolToInt(state.ValidationValid),
		state.ValidationCheckedAt,
		state.ValidationIssueCount,
		normalizedTemplateSourceInfo(state.Source).Type,
		nullableString(normalizedTemplateSourceInfo(state.Source).PluginID),
		nullableString(normalizedTemplateSourceInfo(state.Source).LocalID),
	)
	if err != nil {
		return fmt.Errorf("insert render template state %s: %w", state.TemplateID, err)
	}
	return nil
}

func upsertTemplateState(ctx context.Context, tx *sql.Tx, state storedTemplateState) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE render_template_states
		SET current_revision_id = ?, updated_at = ?, validation_valid = ?, validation_checked_at = ?, validation_issue_count = ?, source_type = ?, source_plugin_id = ?, source_local_id = ?
		WHERE template_id = ?`,
		state.CurrentRevisionID,
		state.UpdatedAt,
		boolToInt(state.ValidationValid),
		state.ValidationCheckedAt,
		state.ValidationIssueCount,
		normalizedTemplateSourceInfo(state.Source).Type,
		nullableString(normalizedTemplateSourceInfo(state.Source).PluginID),
		nullableString(normalizedTemplateSourceInfo(state.Source).LocalID),
		state.TemplateID,
	)
	if err != nil {
		return fmt.Errorf("update render template state %s: %w", state.TemplateID, err)
	}
	return nil
}

func loadTemplateStateTx(ctx context.Context, tx *sql.Tx, templateID string) (storedTemplateState, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT template_id, current_revision_id, updated_at, validation_valid, validation_checked_at, validation_issue_count, source_type, source_plugin_id, source_local_id
		FROM render_template_states
		WHERE template_id = ?`, templateID)

	var state storedTemplateState
	var sourcePluginID sql.NullString
	var sourceLocalID sql.NullString
	if err := row.Scan(
		&state.TemplateID,
		&state.CurrentRevisionID,
		&state.UpdatedAt,
		&state.ValidationValid,
		&state.ValidationCheckedAt,
		&state.ValidationIssueCount,
		&state.Source.Type,
		&sourcePluginID,
		&sourceLocalID,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storedTemplateState{}, sql.ErrNoRows
		}
		return storedTemplateState{}, fmt.Errorf("query render template state %s: %w", templateID, err)
	}

	if sourcePluginID.Valid {
		state.Source.PluginID = sourcePluginID.String
	}
	if sourceLocalID.Valid {
		state.Source.LocalID = sourceLocalID.String
	}
	return state, nil
}
