package storage

// LogTimestampExpression preserves historical offsets and fractional precision.
// SQLite normalizes whole seconds; text padding retains nanosecond ordering.
// This expression belongs to the historical 000005/000006 indexes.
const LogTimestampExpression = `(CASE WHEN length(ts) = 30 AND substr(ts, -1) = 'Z' THEN ts ELSE
 strftime('%Y-%m-%dT%H:%M:%S', substr(ts, 1, 19) ||
   CASE WHEN substr(ts, -1) = 'Z' THEN 'Z' ELSE substr(ts, -6) END) || '.' ||
 substr((CASE WHEN substr(ts, 20, 1) = '.' THEN
   substr(ts, 21, length(ts) - 20 - CASE WHEN substr(ts, -1) = 'Z' THEN 1 ELSE 6 END)
   ELSE '' END) || '000000000', 1, 9) || 'Z' END)`

const logTimeIndexesSchema = `
DROP INDEX idx_management_logs_ts;
DROP INDEX idx_management_logs_plugin;
DROP INDEX idx_management_logs_request;
DROP INDEX idx_management_logs_source;
DROP INDEX idx_management_logs_boot_ts;
CREATE INDEX idx_management_logs_ts ON management_logs (` + LogTimestampExpression + ` DESC, id DESC);
CREATE INDEX idx_management_logs_plugin ON management_logs (plugin_id, ` + LogTimestampExpression + ` DESC, id DESC);
CREATE INDEX idx_management_logs_request ON management_logs (request_id, ` + LogTimestampExpression + ` DESC, id DESC);
CREATE INDEX idx_management_logs_source ON management_logs (source, ` + LogTimestampExpression + ` DESC, id DESC);
CREATE INDEX idx_management_logs_boot_ts ON management_logs (boot_id, ` + LogTimestampExpression + ` DESC, id DESC);
CREATE INDEX idx_management_logs_prune ON management_logs ((CASE WHEN ts GLOB '[0-9]*' THEN julianday(ts) ELSE -1 END));
`
