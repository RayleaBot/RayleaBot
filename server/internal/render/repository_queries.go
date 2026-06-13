package render

import (
	"context"
	"database/sql"
	"fmt"
)

func (r *sqliteTemplateRepository) ListTemplateSummaries(ctx context.Context) ([]TemplateSummary, error) {
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
