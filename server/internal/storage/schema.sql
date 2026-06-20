CREATE TABLE IF NOT EXISTS auth_bootstrap_state (
    singleton_id INTEGER PRIMARY KEY CHECK (singleton_id = 1),
    identifier TEXT NOT NULL,
    secret_digest BLOB NOT NULL,
    signing_key BLOB NOT NULL,
    initialized_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS admin_sessions (
    session_id TEXT PRIMARY KEY,
    subject TEXT NOT NULL,
    issued_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_admin_sessions_expires_at
    ON admin_sessions (expires_at);

CREATE TABLE IF NOT EXISTS plugin_instances (
    plugin_id TEXT PRIMARY KEY,
    desired_state TEXT NOT NULL CHECK (desired_state IN ('enabled', 'disabled')),
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS plugin_packages (
    plugin_id TEXT PRIMARY KEY,
    source_type TEXT NOT NULL CHECK (source_type IN ('local_directory', 'local_zip', 'remote_url')),
    source_ref TEXT NOT NULL,
    version TEXT NOT NULL,
    manifest_hash TEXT NOT NULL,
    package_hash TEXT NOT NULL,
    installed_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tasks (
    task_id TEXT PRIMARY KEY,
    task_type TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled', 'interrupted')),
    progress INTEGER NOT NULL DEFAULT 0,
    summary TEXT NOT NULL DEFAULT '',
    started_at TEXT,
    finished_at TEXT,
    result_json TEXT,
    error_json TEXT,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks (status);
CREATE INDEX IF NOT EXISTS idx_tasks_task_type ON tasks (task_type);
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks (created_at);

CREATE TABLE IF NOT EXISTS secret_store (
    key TEXT PRIMARY KEY,
    value BLOB NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS scheduler_jobs (
    job_id TEXT PRIMARY KEY,
    plugin_id TEXT NOT NULL,
    log_label TEXT NOT NULL DEFAULT '',
    cron_expr TEXT NOT NULL,
    payload TEXT NOT NULL DEFAULT '{}',
    enabled INTEGER NOT NULL DEFAULT 1,
    next_run TEXT NOT NULL,
    last_run TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    last_duration_ms INTEGER NOT NULL DEFAULT 0,
    last_error_code TEXT NOT NULL DEFAULT '',
    last_error_message TEXT NOT NULL DEFAULT '',
    last_error_at TEXT,
    success_count INTEGER NOT NULL DEFAULT 0,
    failure_count INTEGER NOT NULL DEFAULT 0,
    timeout_count INTEGER NOT NULL DEFAULT 0,
    retry_count INTEGER NOT NULL DEFAULT 0,
    other_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_scheduler_jobs_next_run
    ON scheduler_jobs (next_run) WHERE enabled = 1;

CREATE INDEX IF NOT EXISTS idx_scheduler_jobs_plugin_id
    ON scheduler_jobs (plugin_id);

CREATE TABLE IF NOT EXISTS blacklist_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_type TEXT NOT NULL CHECK (entry_type IN ('user', 'group')),
    target_id TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    UNIQUE(entry_type, target_id)
);

CREATE INDEX IF NOT EXISTS idx_blacklist_entries_lookup
    ON blacklist_entries (entry_type, target_id);

CREATE TABLE IF NOT EXISTS management_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    log_id TEXT NOT NULL,
    boot_id TEXT NOT NULL DEFAULT '',
    ts TEXT NOT NULL,
    level TEXT NOT NULL CHECK (level IN ('debug', 'info', 'warn', 'error')),
    source TEXT NOT NULL,
    message TEXT NOT NULL,
    plugin_id TEXT NOT NULL DEFAULT '',
    request_id TEXT NOT NULL DEFAULT '',
    details_json TEXT NOT NULL DEFAULT '{}'
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_management_logs_log_id
    ON management_logs (log_id);

CREATE INDEX IF NOT EXISTS idx_management_logs_ts
    ON management_logs (ts DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_management_logs_plugin
    ON management_logs (plugin_id, ts DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_management_logs_request
    ON management_logs (request_id, ts DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_management_logs_source
    ON management_logs (source, ts DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_management_logs_boot_ts
    ON management_logs (boot_id, ts DESC, id DESC);

CREATE TABLE IF NOT EXISTS plugin_kv (
    plugin_id TEXT NOT NULL,
    key TEXT NOT NULL,
    value_json TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (plugin_id, key)
);

CREATE INDEX IF NOT EXISTS idx_plugin_kv_plugin_id
    ON plugin_kv (plugin_id);

CREATE TABLE IF NOT EXISTS system_configs (
    namespace TEXT NOT NULL,
    key TEXT NOT NULL,
    value_json TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (namespace, key)
);

CREATE INDEX IF NOT EXISTS idx_system_configs_namespace
    ON system_configs (namespace);

CREATE TABLE IF NOT EXISTS third_party_accounts (
    platform TEXT NOT NULL CHECK (platform IN ('bilibili', 'weibo', 'douyin', 'netease_music')),
    account_id TEXT NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    secret_key TEXT NOT NULL,
    profile_uid TEXT NOT NULL DEFAULT '',
    profile_nickname TEXT NOT NULL DEFAULT '',
    profile_avatar_url TEXT NOT NULL DEFAULT '',
    credential_state TEXT NOT NULL DEFAULT 'unknown' CHECK (credential_state IN ('unknown', 'valid', 'invalid')),
    credential_checked_at TEXT,
    credential_last_error TEXT NOT NULL DEFAULT '',
    last_used_at TEXT,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (platform, account_id)
);

CREATE INDEX IF NOT EXISTS idx_third_party_accounts_platform
    ON third_party_accounts (platform);

CREATE TABLE IF NOT EXISTS bilibili_source_config (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    ua_rotation_enabled INTEGER NOT NULL DEFAULT 1 CHECK (ua_rotation_enabled IN (0, 1)),
    fingerprint_enabled INTEGER NOT NULL DEFAULT 1 CHECK (fingerprint_enabled IN (0, 1)),
    dm_img_enabled INTEGER NOT NULL DEFAULT 1 CHECK (dm_img_enabled IN (0, 1)),
    captcha_recovery_enabled INTEGER NOT NULL DEFAULT 0 CHECK (captcha_recovery_enabled IN (0, 1)),
    proxy_global_enabled INTEGER NOT NULL DEFAULT 0 CHECK (proxy_global_enabled IN (0, 1)),
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS bilibili_source_rooms (
    uid TEXT PRIMARY KEY,
    room_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL DEFAULT '',
    face TEXT NOT NULL DEFAULT '',
    cover_url TEXT NOT NULL DEFAULT '',
    live_status INTEGER NOT NULL DEFAULT 0 CHECK (live_status IN (0, 1)),
    live_started_at INTEGER NOT NULL DEFAULT 0,
    live_event_id TEXT NOT NULL DEFAULT '',
    connection_state TEXT NOT NULL DEFAULT 'idle' CHECK (connection_state IN ('idle', 'connecting', 'connected', 'degraded', 'failed')),
    last_event_at TEXT,
    last_error TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_bilibili_source_rooms_state
    ON bilibili_source_rooms (connection_state);

CREATE TABLE IF NOT EXISTS bilibili_source_seen (
    event_key TEXT PRIMARY KEY,
    uid TEXT NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN ('bilibili.live.started', 'bilibili.live.ended', 'bilibili.dynamic.published')),
    source_id TEXT NOT NULL,
    observed_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_bilibili_source_seen_uid
    ON bilibili_source_seen (uid, observed_at DESC);

CREATE TABLE IF NOT EXISTS bilibili_source_dynamics (
    uid TEXT PRIMARY KEY,
    dynamic_id TEXT NOT NULL,
    service TEXT NOT NULL CHECK (service IN ('video', 'image_text', 'article', 'repost')),
    title TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    username TEXT NOT NULL DEFAULT '',
    avatar_url TEXT NOT NULL DEFAULT '',
    images_json TEXT NOT NULL DEFAULT '[]',
    published_at INTEGER NOT NULL DEFAULT 0,
    observed_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_bilibili_source_dynamics_observed_at
    ON bilibili_source_dynamics (observed_at DESC);

CREATE TABLE IF NOT EXISTS bilibili_source_state (
    key TEXT PRIMARY KEY,
    value_json TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS render_template_revisions (
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

CREATE INDEX IF NOT EXISTS idx_render_template_revisions_template_saved_at
    ON render_template_revisions (template_id, saved_at DESC, revision_id DESC);

CREATE INDEX IF NOT EXISTS idx_render_template_revisions_template_digest
    ON render_template_revisions (template_id, source_digest);

CREATE TABLE IF NOT EXISTS render_template_states (
    template_id TEXT PRIMARY KEY,
    current_revision_id TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    validation_valid INTEGER NOT NULL CHECK (validation_valid IN (0, 1)),
    validation_checked_at TEXT NOT NULL,
    validation_issue_count INTEGER NOT NULL CHECK (validation_issue_count >= 0),
    source_type TEXT NOT NULL DEFAULT 'system' CHECK (source_type IN ('system', 'plugin')),
    source_plugin_id TEXT,
    source_local_id TEXT,
    FOREIGN KEY (current_revision_id) REFERENCES render_template_revisions (revision_id)
);

CREATE INDEX IF NOT EXISTS idx_render_template_states_source
    ON render_template_states (source_type, source_plugin_id, source_local_id);

CREATE TABLE IF NOT EXISTS whitelist_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_type TEXT NOT NULL CHECK (entry_type IN ('user', 'group')),
    target_id TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    UNIQUE(entry_type, target_id)
);

CREATE INDEX IF NOT EXISTS idx_whitelist_entries_lookup
    ON whitelist_entries (entry_type, target_id);

CREATE TABLE IF NOT EXISTS whitelist_state (
    singleton_id INTEGER PRIMARY KEY CHECK (singleton_id = 1),
    enabled INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
    updated_at TEXT NOT NULL
);

INSERT OR IGNORE INTO whitelist_state (singleton_id, enabled, updated_at)
VALUES (1, 0, '1970-01-01T00:00:00Z');
