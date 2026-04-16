import type { LogListResponse, LogPageDirection, LogProtocol, LogSummary } from '@/types/api'

export type LogScope = 'history' | 'current_session'

export const DEFAULT_LOG_PAGE_LIMIT = 100
export const MAX_LOG_PAGE_LIMIT = 200

export interface LogFilters {
  level?: string
  source?: string
  protocol?: LogProtocol
  pluginId?: string
  requestId?: string
}

export interface HistoryTimeRange {
  startAt?: string | null
  endAt?: string | null
}

interface BuildLogListPathOptions {
  scope: LogScope
  filters?: LogFilters
  timeRange?: HistoryTimeRange
  cursor?: string | null
  direction?: LogPageDirection
  limit?: number
}

export function buildLogListPath(options: BuildLogListPathOptions) {
  const params = new URLSearchParams()
  const filters = options.filters ?? {}

  params.set('scope', options.scope)
  params.set('limit', String(normalizeLogLimit(options.limit)))

  const level = normalizeFilterValue(filters.level)
  const source = normalizeFilterValue(filters.source)
  const protocol = normalizeFilterValue(filters.protocol)
  const pluginId = normalizeFilterValue(filters.pluginId)
  const requestId = normalizeFilterValue(filters.requestId)
  const startAt = normalizeFilterValue(options.timeRange?.startAt)
  const endAt = normalizeFilterValue(options.timeRange?.endAt)
  const cursor = normalizeFilterValue(options.cursor)

  if (level) {
    params.set('level', level)
  }
  if (source) {
    params.set('source', source)
  }
  if (protocol) {
    params.set('protocol', protocol)
  }
  if (pluginId) {
    params.set('plugin_id', pluginId)
  }
  if (requestId) {
    params.set('request_id', requestId)
  }
  if (options.scope === 'history' && startAt) {
    params.set('start_at', startAt)
  }
  if (options.scope === 'history' && endAt) {
    params.set('end_at', endAt)
  }
  if (cursor) {
    params.set('cursor', cursor)
  }
  if (options.direction) {
    params.set('direction', options.direction)
  }

  return `/api/logs?${params.toString()}`
}

export function matchesLogFilters(log: LogSummary, filters: LogFilters) {
  if (filters.level && log.level !== filters.level) {
    return false
  }
  if (filters.source && log.source !== filters.source) {
    return false
  }
  if (filters.protocol && log.protocol !== filters.protocol) {
    return false
  }
  if (filters.pluginId && log.plugin_id !== filters.pluginId) {
    return false
  }
  if (filters.requestId && log.request_id !== filters.requestId) {
    return false
  }
  return true
}

export function normalizeLogLimit(limit: number | undefined, fallback = DEFAULT_LOG_PAGE_LIMIT) {
  const nextFallback = Number.isFinite(fallback) && fallback > 0
    ? Math.floor(fallback)
    : DEFAULT_LOG_PAGE_LIMIT
  if (!limit || !Number.isFinite(limit) || limit < 1) {
    return Math.min(MAX_LOG_PAGE_LIMIT, nextFallback)
  }
  return Math.min(MAX_LOG_PAGE_LIMIT, Math.floor(limit))
}

export function sortLogItemsAsc(items: LogSummary[]) {
  return [...items].sort((left, right) => {
    const leftTimestamp = toComparableTimestamp(left.timestamp)
    const rightTimestamp = toComparableTimestamp(right.timestamp)
    if (leftTimestamp !== rightTimestamp) {
      return leftTimestamp - rightTimestamp
    }
    return getLogIdentityKey(left).localeCompare(getLogIdentityKey(right))
  })
}

export function mergeLogItemsAsc(existingItems: LogSummary[], nextItems: LogSummary[]) {
  const merged = new Map<string, LogSummary>()

  for (const item of [...existingItems, ...nextItems]) {
    merged.set(getLogIdentityKey(item), item)
  }

  return sortLogItemsAsc(Array.from(merged.values()))
}

export function normalizeLogListResponseItems(response: LogListResponse | null | undefined) {
  return sortLogItemsAsc(response?.items ?? [])
}

export function getLogIdentityKey(log: LogSummary) {
  return log.log_id || [
    log.timestamp,
    log.level,
    log.source,
    log.protocol ?? '',
    log.plugin_id ?? '',
    log.request_id ?? '',
    log.message,
  ].join('|')
}

function normalizeFilterValue(value: string | undefined | null) {
  const nextValue = value?.trim()
  return nextValue ? nextValue : ''
}

function toComparableTimestamp(value: string) {
  const numeric = Number(value)
  if (Number.isFinite(numeric) && numeric > 0) {
    return numeric >= 1_000_000_000_000 ? numeric : numeric * 1000
  }

  const parsed = Date.parse(value)
  if (Number.isFinite(parsed)) {
    return parsed
  }

  return 0
}
