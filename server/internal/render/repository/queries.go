package repository

import (
	"context"
	"database/sql"
	"fmt"
)

func (r *SQLiteTemplateRepository) ListTemplateSummaries(ctx context.Context) ([]TemplateSummary, error) {
	rows, err := r.read.QueryContext(ctx, `
		SELECT
			s.template_id,
			s.current_revision_id,
			s.updated_at,
			s.source_type,
			s.source_plugin_id,
			s.source_local_id,
			r.template_version,
			r.manifest_json,
			r.input_schema_json
		FROM render_template_states s
		INNER JOIN render_template_revisions r ON r.revision_id = s.current_revision_id
		ORDER BY s.template_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query render template summaries: %w", err)
	}
	defer rows.Close()

	var items []TemplateSummary
	for rows.Next() {
		var (
			templateID        string
			currentRevisionID string
			updatedAt         string
			source            TemplateSourceInfo
			sourcePluginID    sql.NullString
			sourceLocalID     sql.NullString
			templateVersion   string
			manifestJSONText  string
			inputSchemaJSON   sql.NullString
		)
		if err := rows.Scan(&templateID, &currentRevisionID, &updatedAt, &source.Type, &sourcePluginID, &sourceLocalID, &templateVersion, &manifestJSONText, &inputSchemaJSON); err != nil {
			return nil, fmt.Errorf("scan render template summary: %w", err)
		}
		if sourcePluginID.Valid {
			source.PluginID = sourcePluginID.String
		}
		if sourceLocalID.Valid {
			source.LocalID = sourceLocalID.String
		}

		manifest, err := decodeStoredManifest(templateID, manifestJSONText)
		if err != nil {
			return nil, err
		}

		items = append(items, TemplateSummary{
			ID:                templateID,
			Version:           templateVersion,
			Width:             manifest.Width,
			Height:            manifest.Height,
			HasInputSchema:    inputSchemaJSON.Valid && inputSchemaJSON.String != "",
			CurrentRevisionID: currentRevisionID,
			UpdatedAt:         updatedAt,
			Source:            normalizedTemplateSourceInfo(source),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate render template summaries: %w", err)
	}

	return items, nil
}

func (r *SQLiteTemplateRepository) ListTemplateVersions(ctx context.Context, templateID string) ([]TemplateVersion, error) {
	if exists, err := r.TemplateExists(ctx, templateID); err != nil {
		return nil, err
	} else if !exists {
		return nil, sql.ErrNoRows
	}

	rows, err := r.read.QueryContext(ctx, `
		SELECT revision_id, template_version, saved_at, kind, message
		FROM render_template_revisions
		WHERE template_id = ?
		ORDER BY saved_at DESC, revision_id DESC`, templateID)
	if err != nil {
		return nil, fmt.Errorf("query render template versions for %s: %w", templateID, err)
	}
	defer rows.Close()

	var versions []TemplateVersion
	for rows.Next() {
		var (
			version TemplateVersion
			message sql.NullString
		)
		if err := rows.Scan(&version.RevisionID, &version.TemplateVersion, &version.SavedAt, &version.Kind, &message); err != nil {
			return nil, fmt.Errorf("scan render template version for %s: %w", templateID, err)
		}
		version.Message = nullStringPointer(message)
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate render template versions for %s: %w", templateID, err)
	}

	return versions, nil
}
