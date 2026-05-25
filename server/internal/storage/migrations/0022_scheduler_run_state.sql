ALTER TABLE scheduler_jobs ADD COLUMN last_duration_ms INTEGER NOT NULL DEFAULT 0;
ALTER TABLE scheduler_jobs ADD COLUMN last_error_code TEXT NOT NULL DEFAULT '';
ALTER TABLE scheduler_jobs ADD COLUMN last_error_message TEXT NOT NULL DEFAULT '';
ALTER TABLE scheduler_jobs ADD COLUMN last_error_at TEXT;
ALTER TABLE scheduler_jobs ADD COLUMN success_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE scheduler_jobs ADD COLUMN failure_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE scheduler_jobs ADD COLUMN timeout_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE scheduler_jobs ADD COLUMN retry_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE scheduler_jobs ADD COLUMN other_count INTEGER NOT NULL DEFAULT 0;
