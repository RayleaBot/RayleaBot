-- Template files are authoritative. Discard the obsolete editor/history cache.
DROP TABLE IF EXISTS render_template_states;
DROP TABLE IF EXISTS render_template_revisions;

CREATE TABLE IF NOT EXISTS render_templates (
    template_id TEXT PRIMARY KEY,
    source_digest TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    source_type TEXT NOT NULL CHECK (source_type IN ('system', 'plugin')),
    source_plugin_id TEXT,
    source_local_id TEXT,
    manifest_json TEXT NOT NULL,
    html TEXT NOT NULL,
    stylesheet TEXT NOT NULL,
    input_schema_json TEXT
);

CREATE INDEX IF NOT EXISTS idx_render_templates_source
    ON render_templates (source_type, source_plugin_id);
