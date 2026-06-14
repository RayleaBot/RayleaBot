package repository

import (
	"context"
	"fmt"
	"strings"
)

func (r *SQLiteTemplateRepository) RemovePluginTemplatesExcept(ctx context.Context, pluginID string, keepIDs []string) error {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return nil
	}

	args := []any{pluginID}
	query := `DELETE FROM render_template_states WHERE source_type = 'plugin' AND source_plugin_id = ?`
	if len(keepIDs) > 0 {
		placeholders := make([]string, 0, len(keepIDs))
		seen := make(map[string]struct{}, len(keepIDs))
		for _, templateID := range keepIDs {
			templateID = strings.TrimSpace(templateID)
			if templateID == "" {
				continue
			}
			if _, ok := seen[templateID]; ok {
				continue
			}
			seen[templateID] = struct{}{}
			placeholders = append(placeholders, "?")
			args = append(args, templateID)
		}
		if len(placeholders) > 0 {
			query += ` AND template_id NOT IN (` + strings.Join(placeholders, ",") + `)`
		}
	}

	if _, err := r.write.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("remove stale plugin render templates for %s: %w", pluginID, err)
	}
	return nil
}

func (r *SQLiteTemplateRepository) RemovePluginTemplatesNotIn(ctx context.Context, activePluginIDs []string) error {
	seen := map[string]struct{}{}
	for _, pluginID := range activePluginIDs {
		pluginID = strings.TrimSpace(pluginID)
		if pluginID == "" {
			continue
		}
		seen[pluginID] = struct{}{}
	}
	if len(seen) == 0 {
		if _, err := r.write.ExecContext(ctx, `DELETE FROM render_template_states WHERE source_type = 'plugin'`); err != nil {
			return fmt.Errorf("remove all plugin render templates: %w", err)
		}
		return nil
	}

	args := make([]any, 0, len(seen))
	placeholders := make([]string, 0, len(seen))
	for pluginID := range seen {
		placeholders = append(placeholders, "?")
		args = append(args, pluginID)
	}
	if _, err := r.write.ExecContext(ctx,
		`DELETE FROM render_template_states WHERE source_type = 'plugin' AND source_plugin_id NOT IN (`+strings.Join(placeholders, ",")+`)`,
		args...,
	); err != nil {
		return fmt.Errorf("remove inactive plugin render templates: %w", err)
	}
	return nil
}
