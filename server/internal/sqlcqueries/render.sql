-- name: UpdateRenderTemplateValidation :execresult
UPDATE render_template_states
SET validation_valid = ?, validation_checked_at = ?, validation_issue_count = ?
WHERE template_id = ?;

-- name: DeletePluginRenderTemplates :exec
DELETE FROM render_template_states
WHERE source_type = 'plugin' AND source_plugin_id = ?;

-- name: DeleteAllPluginRenderTemplates :exec
DELETE FROM render_template_states
WHERE source_type = 'plugin';

-- name: GetRenderTemplateSyncState :one
SELECT
    s.current_revision_id,
    r.source_digest,
    s.validation_valid,
    s.validation_issue_count,
    s.source_type,
    s.source_plugin_id,
    s.source_local_id
FROM render_template_states AS s
INNER JOIN render_template_revisions AS r ON r.revision_id = s.current_revision_id
WHERE s.template_id = ?;

-- name: UpdateRenderTemplateSyncMetadata :exec
UPDATE render_template_states
SET
    validation_valid = ?,
    validation_checked_at = ?,
    validation_issue_count = ?,
    source_type = ?,
    source_plugin_id = ?,
    source_local_id = ?
WHERE template_id = ?;
