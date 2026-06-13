package render

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (r *sqliteTemplateRepository) loadCurrentTemplate(ctx context.Context, templateID string) (storedTemplateState, storedTemplateRevision, error) {
	row := r.read.QueryRowContext(ctx, `
		SELECT
			s.template_id,
			s.current_revision_id,
			s.updated_at,
			s.validation_valid,
			s.validation_checked_at,
			s.validation_issue_count,
			s.source_type,
			s.source_plugin_id,
			s.source_local_id,
			r.revision_id,
			r.template_version,
			r.kind,
			r.message,
			r.saved_at,
			r.source_digest,
			r.manifest_json,
			r.html,
			r.stylesheet,
			r.input_schema_json
		FROM render_template_states s
		INNER JOIN render_template_revisions r ON r.revision_id = s.current_revision_id
		WHERE s.template_id = ?`, templateID)

	var (
		state          storedTemplateState
		revision       storedTemplateRevision
		message        sql.NullString
		inputSchema    sql.NullString
		sourcePluginID sql.NullString
		sourceLocalID  sql.NullString
	)
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
		&revision.RevisionID,
		&revision.TemplateVersion,
		&revision.Kind,
		&message,
		&revision.SavedAt,
		&revision.SourceDigest,
		&revision.ManifestJSON,
		&revision.HTML,
		&revision.Stylesheet,
		&inputSchema,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storedTemplateState{}, storedTemplateRevision{}, sql.ErrNoRows
		}
		return storedTemplateState{}, storedTemplateRevision{}, fmt.Errorf("query render template %s: %w", templateID, err)
	}

	if sourcePluginID.Valid {
		state.Source.PluginID = sourcePluginID.String
	}
	if sourceLocalID.Valid {
		state.Source.LocalID = sourceLocalID.String
	}
	revision.TemplateID = templateID
	revision.Message = nullStringPointer(message)
	revision.InputSchemaJSON = inputSchema
	return state, revision, nil
}

func (r *sqliteTemplateRepository) loadRevision(ctx context.Context, templateID, revisionID string) (storedTemplateRevision, error) {
	row := r.read.QueryRowContext(ctx, `
		SELECT revision_id, template_id, template_version, kind, message, saved_at, source_digest, manifest_json, html, stylesheet, input_schema_json
		FROM render_template_revisions
		WHERE template_id = ? AND revision_id = ?`, templateID, revisionID)

	var (
		revision    storedTemplateRevision
		message     sql.NullString
		inputSchema sql.NullString
	)
	if err := row.Scan(
		&revision.RevisionID,
		&revision.TemplateID,
		&revision.TemplateVersion,
		&revision.Kind,
		&message,
		&revision.SavedAt,
		&revision.SourceDigest,
		&revision.ManifestJSON,
		&revision.HTML,
		&revision.Stylesheet,
		&inputSchema,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storedTemplateRevision{}, sql.ErrNoRows
		}
		return storedTemplateRevision{}, fmt.Errorf("query render template revision %s/%s: %w", templateID, revisionID, err)
	}

	revision.Message = nullStringPointer(message)
	revision.InputSchemaJSON = inputSchema
	return revision, nil
}

func (r *sqliteTemplateRepository) templateExists(ctx context.Context, templateID string) (bool, error) {
	var count int
	if err := r.read.QueryRowContext(ctx, `SELECT COUNT(*) FROM render_template_states WHERE template_id = ?`, templateID).Scan(&count); err != nil {
		return false, fmt.Errorf("query render template existence for %s: %w", templateID, err)
	}
	return count > 0, nil
}
