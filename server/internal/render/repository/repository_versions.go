package repository

import (
	"context"
	"database/sql"
	"fmt"
)

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
