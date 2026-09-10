function normalizeSortableTimestamp(value) {
  if (value === null || value === undefined) {
    return Number.NEGATIVE_INFINITY
  }

  if (typeof value === 'number' && Number.isFinite(value)) {
    return normalizeUnixTimestamp(value)
  }

  const raw = String(value).trim()
  if (!raw) {
    return Number.NEGATIVE_INFINITY
  }

  const numeric = Number(raw)
  if (Number.isFinite(numeric)) {
    return normalizeUnixTimestamp(numeric)
  }

  const parsed = Date.parse(raw)
  return Number.isFinite(parsed) ? parsed : Number.NEGATIVE_INFINITY
}

function normalizeUnixTimestamp(value) {
  const absolute = Math.abs(value)
  if (absolute >= 1_000_000_000 && absolute < 1_000_000_000_000) {
    return value * 1000
  }
  return value
}

function compareLogsDesc(left, right) {
  const leftTimestamp = normalizeSortableTimestamp(left.timestamp)
  const rightTimestamp = normalizeSortableTimestamp(right.timestamp)

  if (leftTimestamp !== rightTimestamp) {
    return rightTimestamp - leftTimestamp
  }

  return String(right.log_id ?? '').localeCompare(String(left.log_id ?? ''))
}

function encodeLogCursor(item) {
  return Buffer.from(JSON.stringify({
    log_id: item.log_id,
  }), 'utf8').toString('base64url')
}

function decodeLogCursor(raw) {
  if (!raw) {
    return null
  }

  try {
    const decoded = JSON.parse(Buffer.from(raw, 'base64url').toString('utf8'))
    return typeof decoded?.log_id === 'string' ? decoded.log_id : null
  } catch {
    return null
  }
}

function listLogPage(state, searchParams) {
  const scope = searchParams.get('scope') === 'current_session' ? 'current_session' : 'history'
  const startAt = searchParams.get('start_at')
  const endAt = searchParams.get('end_at')
  const levels = normalizedSearchValues(searchParams, 'level')
  const source = searchParams.get('source')
  const protocol = searchParams.get('protocol')
  const pluginIds = normalizedSearchValues(searchParams, 'plugin_id')
  const requestId = searchParams.get('request_id')
  const limit = Math.max(1, Number(searchParams.get('limit') ?? '50') || 50)
  const direction = searchParams.get('direction') === 'newer' ? 'newer' : 'older'
  const cursorLogId = decodeLogCursor(searchParams.get('cursor'))

  const filtered = state.logs
    .filter((item) => {
      const timestamp = normalizeSortableTimestamp(item.timestamp)
      if (scope === 'current_session' && !state.currentSessionLogIds.has(item.log_id)) return false
      if (scope === 'history' && startAt && timestamp < normalizeSortableTimestamp(startAt)) return false
      if (scope === 'history' && endAt && timestamp > normalizeSortableTimestamp(endAt)) return false
      if (levels.length > 0 && !levels.includes(item.level)) return false
      if (source && item.source !== source) return false
      if (protocol && item.protocol !== protocol) return false
      if (pluginIds.length > 0 && !pluginIds.includes(item.plugin_id ?? '')) return false
      if (requestId && item.request_id !== requestId) return false
      return true
    })
    .slice()
    .sort(compareLogsDesc)

  let startIndex = 0
  let endIndex = Math.min(limit, filtered.length)
  const cursorIndex = cursorLogId
    ? filtered.findIndex((item) => item.log_id === cursorLogId)
    : -1

  if (cursorIndex >= 0) {
    if (direction === 'older') {
      startIndex = cursorIndex + 1
      endIndex = Math.min(filtered.length, startIndex + limit)
    } else {
      endIndex = cursorIndex
      startIndex = Math.max(0, endIndex - limit)
    }
  }

  const items = filtered.slice(startIndex, endIndex)
  const hasNewer = startIndex > 0
  const hasOlder = endIndex < filtered.length

  return {
    items,
    page: {
      limit,
      has_older: hasOlder,
      has_newer: hasNewer,
      older_cursor: hasOlder && items.length > 0 ? encodeLogCursor(items.at(-1)) : null,
      newer_cursor: hasNewer && items.length > 0 ? encodeLogCursor(items[0]) : null,
    },
  }
}

function normalizedSearchValues(searchParams, key) {
  return Array.from(new Set(
    searchParams
      .getAll(key)
      .map((value) => value.trim())
      .filter(Boolean),
  ))
}

export { listLogPage }
