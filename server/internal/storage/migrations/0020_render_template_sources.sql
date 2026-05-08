ALTER TABLE render_template_states
ADD COLUMN source_type TEXT NOT NULL DEFAULT 'system' CHECK (source_type IN ('system', 'plugin'));

ALTER TABLE render_template_states
ADD COLUMN source_plugin_id TEXT;

ALTER TABLE render_template_states
ADD COLUMN source_local_id TEXT;

CREATE INDEX idx_render_template_states_source
ON render_template_states(source_type, source_plugin_id, source_local_id);
