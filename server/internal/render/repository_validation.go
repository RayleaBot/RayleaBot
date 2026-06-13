package render

import (
	"context"
	"database/sql"
	"fmt"
)

func (r *sqliteTemplateRepository) UpdateValidationStatus(ctx context.Context, templateID string, validation TemplateValidationStatus) error {
	result, err := r.write.ExecContext(ctx, `
		UPDATE render_template_states
		SET validation_valid = ?, validation_checked_at = ?, validation_issue_count = ?
		WHERE template_id = ?`,
		boolToInt(validation.Valid),
		validation.CheckedAt,
		validation.IssueCount,
		templateID,
	)
	if err != nil {
		return fmt.Errorf("update render template validation for %s: %w", templateID, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read render template validation update rows for %s: %w", templateID, err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}
