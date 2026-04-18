CREATE TABLE render_template_revisions (
	revision_id TEXT PRIMARY KEY,
	template_id TEXT NOT NULL,
	template_version TEXT NOT NULL,
	kind TEXT NOT NULL CHECK (kind IN ('save', 'rollback')),
	message TEXT,
	saved_at TEXT NOT NULL,
	source_digest TEXT NOT NULL,
	manifest_json TEXT NOT NULL,
	html TEXT NOT NULL,
	stylesheet TEXT NOT NULL,
	input_schema_json TEXT
);

CREATE INDEX idx_render_template_revisions_template_saved_at
ON render_template_revisions(template_id, saved_at DESC, revision_id DESC);

CREATE INDEX idx_render_template_revisions_template_digest
ON render_template_revisions(template_id, source_digest);

CREATE TABLE render_template_states (
	template_id TEXT PRIMARY KEY,
	current_revision_id TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	validation_valid INTEGER NOT NULL CHECK (validation_valid IN (0, 1)),
	validation_checked_at TEXT NOT NULL,
	validation_issue_count INTEGER NOT NULL CHECK (validation_issue_count >= 0),
	FOREIGN KEY (current_revision_id) REFERENCES render_template_revisions(revision_id)
);
