// Presentation data only: these cursors identify rows in the test's in-memory
// dataset. They deliberately cannot encode/decode a Server log cursor.
const prefix = 'fixture-row:'
const matchingKeys = ['level', 'source', 'protocol', 'plugin_id', 'request_id']

export function listLogPage(state, query) {
  const history = query.get('scope') !== 'current_session'
  const start = query.get('start_at')
  const end = query.get('end_at')
  const rows = state.logs.filter(row => {
    if (!history && !state.currentSessionLogIds.has(row.log_id)) return false
    if (history && start && row.timestamp < start) return false
    if (history && end && row.timestamp > end) return false
    return matchingKeys.every(key => !query.has(key) || query.getAll(key).includes(row[key]))
  }).sort((a, b) => String(b.timestamp).localeCompare(String(a.timestamp)) || b.log_id.localeCompare(a.log_id))
  const limit = Number(query.get('limit') ?? 50)
  const cursor = query.get('cursor')
  const anchor = cursor?.startsWith(prefix) ? rows.findIndex(row => row.log_id === cursor.slice(prefix.length)) : -1
  const newer = query.get('direction') === 'newer'
  const from = anchor < 0 ? 0 : newer ? Math.max(0, anchor - limit) : anchor + 1
  const to = anchor >= 0 && newer ? anchor : Math.min(rows.length, from + limit)
  const items = rows.slice(from, to)
  return {
    items,
    page: {
      limit,
      has_older: to < rows.length,
      has_newer: from > 0,
      older_cursor: to < rows.length && items.length ? prefix + items.at(-1).log_id : null,
      newer_cursor: from > 0 && items.length ? prefix + items[0].log_id : null,
    },
  }
}
