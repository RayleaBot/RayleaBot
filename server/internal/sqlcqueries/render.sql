-- name: GetRenderTemplate :one
SELECT * FROM render_templates WHERE template_id = ?;

-- name: ListRenderTemplates :many
SELECT * FROM render_templates ORDER BY template_id;

-- name: UpsertRenderTemplate :exec
INSERT INTO render_templates (
    template_id, source_digest, updated_at, source_type,
    source_plugin_id, source_local_id, manifest_json, html, stylesheet, input_schema_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(template_id) DO UPDATE SET
    source_digest = excluded.source_digest,
    updated_at = excluded.updated_at,
    manifest_json = excluded.manifest_json,
    html = excluded.html,
    stylesheet = excluded.stylesheet,
    input_schema_json = excluded.input_schema_json;
