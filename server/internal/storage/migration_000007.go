package storage

import (
	"context"
	"database/sql"
	"math"
	"strconv"
	"strings"
	"time"
)

const logNanosecondsTableSchema = `
CREATE TABLE management_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    log_id TEXT NOT NULL,
    boot_id TEXT NOT NULL DEFAULT '',
    ts INTEGER NOT NULL,
    level TEXT NOT NULL CHECK (level IN ('debug', 'info', 'warn', 'error')),
    source TEXT NOT NULL,
    message TEXT NOT NULL,
    plugin_id TEXT NOT NULL DEFAULT '',
    request_id TEXT NOT NULL DEFAULT '',
    details_json TEXT NOT NULL DEFAULT '{}'
);`

const logNanosecondsIndexesSchema = `
CREATE UNIQUE INDEX idx_management_logs_log_id ON management_logs (log_id);
CREATE INDEX idx_management_logs_ts ON management_logs (ts DESC, id DESC);
CREATE INDEX idx_management_logs_plugin ON management_logs (plugin_id, ts DESC, id DESC);
CREATE INDEX idx_management_logs_request ON management_logs (request_id, ts DESC, id DESC);
CREATE INDEX idx_management_logs_source ON management_logs (source, ts DESC, id DESC);
CREATE INDEX idx_management_logs_boot_ts ON management_logs (boot_id, ts DESC, id DESC);
`

func migrateLogNanoseconds(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE management_logs RENAME TO management_logs_000006;`+logNanosecondsTableSchema); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, ts FROM management_logs_000006 ORDER BY id`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	insert, err := tx.PrepareContext(ctx, `INSERT INTO management_logs
		(id, log_id, boot_id, ts, level, source, message, plugin_id, request_id, details_json)
		SELECT id, log_id, boot_id, ?, level, source, message, plugin_id, request_id, details_json
		FROM management_logs_000006 WHERE id = ?`)
	if err != nil {
		return err
	}
	defer func() { _ = insert.Close() }()
	for rows.Next() {
		var id int64
		var value any
		if err := rows.Scan(&id, &value); err != nil {
			return err
		}
		ts, ok := historicalLogNanoseconds(value)
		if !ok {
			// Discard timestamps without an instant representable as int64 nanoseconds.
			continue
		}
		if _, err := insert.ExecContext(ctx, ts, id); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	// Keep the high-water mark even if the latest rows were previously pruned
	// or discarded here, so an existing cursor never aliases a new row ID.
	_, err = tx.ExecContext(ctx, `
INSERT INTO sqlite_sequence(name, seq)
SELECT 'management_logs', seq FROM sqlite_sequence WHERE name = 'management_logs_000006'
AND NOT EXISTS (SELECT 1 FROM sqlite_sequence WHERE name = 'management_logs');
UPDATE sqlite_sequence SET seq = max(seq, coalesce(
    (SELECT seq FROM sqlite_sequence WHERE name = 'management_logs_000006'), 0))
WHERE name = 'management_logs';
DROP TABLE management_logs_000006;`+logNanosecondsIndexesSchema)
	return err
}

func historicalLogNanoseconds(value any) (int64, bool) {
	var julian float64
	switch value := value.(type) {
	case int64:
		julian = float64(value)
	case float64:
		julian = value
	case string:
		value = strings.TrimSpace(value)
		for _, layout := range []string{
			time.RFC3339Nano, "2006-01-02 15:04:05Z07:00",
			"2006-01-02T15:04:05", "2006-01-02 15:04:05",
			"2006-01-02T15:04Z07:00", "2006-01-02 15:04Z07:00",
			"2006-01-02T15:04", "2006-01-02 15:04", "2006-01-02",
		} {
			if instant, err := time.Parse(layout, value); err == nil {
				ns := instant.UnixNano()
				return ns, time.Unix(0, ns).Equal(instant)
			}
		}
		var err error
		julian, err = strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, false
		}
	default:
		return 0, false
	}
	if math.IsNaN(julian) || math.IsInf(julian, 0) || julian < 0 || julian > 5373484.5 {
		return 0, false
	}
	days, fraction := math.Modf(julian - 2440587.5)
	instant := time.Unix(int64(days)*86400, int64(math.Round(fraction*86400*1e9)))
	ns := instant.UnixNano()
	return ns, time.Unix(0, ns).Equal(instant)
}
