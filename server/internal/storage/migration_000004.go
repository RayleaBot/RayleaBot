package storage

// Message statistics begin when the online collector starts, not when an
// offline CLI opens or migrates the database.
const messageStatsSchema = `
CREATE TABLE message_stats_tracking (
    singleton_id INTEGER PRIMARY KEY CHECK (singleton_id = 1),
    started_at_ms INTEGER NOT NULL
);
CREATE TABLE message_stats_adapters (
    adapter_id TEXT PRIMARY KEY,
    protocol TEXT NOT NULL,
    last_received_at_ms INTEGER
);
CREATE TABLE message_stats_hours (
    hour_start INTEGER NOT NULL,
    adapter_id TEXT NOT NULL,
    received INTEGER NOT NULL CHECK (received >= 0),
    sent INTEGER NOT NULL CHECK (sent >= 0),
    PRIMARY KEY (hour_start, adapter_id)
);
CREATE TABLE message_stats_runs (
    id INTEGER PRIMARY KEY,
    started_at_ms INTEGER NOT NULL,
    last_alive_at_ms INTEGER NOT NULL,
    stopped_at_ms INTEGER
);
CREATE TABLE message_stats_offline (
    run_id INTEGER NOT NULL REFERENCES message_stats_runs(id),
    adapter_id TEXT NOT NULL,
    started_at_ms INTEGER NOT NULL,
    ended_at_ms INTEGER,
    PRIMARY KEY (run_id, adapter_id, started_at_ms)
);
CREATE INDEX idx_message_stats_offline_start ON message_stats_offline(started_at_ms);
`
