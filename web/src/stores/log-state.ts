import type { LogLevel, LogListResponse, LogPageDirection, LogProtocol, LogSummary } from '@/types/api'

export type LogScope = 'history' | 'current_session'

export const DEFAULT_LOG_PAGE_LIMIT = 100
export const MAX_LOG_PAGE_LIMIT = 200

export interface LogFilters {
  level?: LogLevel
  levels?: LogLevel[]
  source?: string
  protocol?: LogProtocol
  pluginId?: string
  pluginIds?: string[]
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

  const levels = normalizeFilterValues(filters.levels, filters.level)
  const source = normalizeFilterValue(filters.source)
  const protocol = normalizeFilterValue(filters.protocol)
  const pluginIds = normalizeFilterValues(filters.pluginIds, filters.pluginId)
  const requestId = normalizeFilterValue(filters.requestId)
  const startAt = normalizeFilterValue(options.timeRange?.startAt)
  const endAt = normalizeFilterValue(options.timeRange?.endAt)
  const cursor = normalizeFilterValue(options.cursor)

  for (const level of levels) {
    params.append('level', level)
  }
  if (source) {
    params.set('source', source)
  }
  if (protocol) {
    params.set('protocol', protocol)
  }
  for (const pluginId of pluginIds) {
    params.append('plugin_id', pluginId)
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
  const levels = normalizeFilterValues(filters.levels, filters.level)
  const pluginIds = normalizeFilterValues(filters.pluginIds, filters.pluginId)

  if (levels.length > 0 && !levels.includes(log.level)) {
    return false
  }
  if (filters.source && log.source !== filters.source) {
    return false
  }
  if (filters.protocol && log.protocol !== filters.protocol) {
    return false
  }
  if (pluginIds.length > 0 && !pluginIds.includes(log.plugin_id ?? '')) {
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

export function mergeSortedLogItemsAsc(existingItems: LogSummary[], nextItems: LogSummary[]) {
  if (existingItems.length === 0) {
    return sortLogItemsAsc(nextItems)
  }
  if (nextItems.length === 0) {
    return [...existingItems]
  }

  const nextMap = new Map<string, LogSummary>()
  for (const item of nextItems) {
    nextMap.set(getLogIdentityKey(item), item)
  }
  const sortedNext = sortLogItemsAsc(Array.from(nextMap.values()))

  const result: LogSummary[] = []
  let i = 0
  let j = 0

  while (i < existingItems.length && j < sortedNext.length) {
    const left = existingItems[i]!
    const right = sortedNext[j]!
    const leftTs = toComparableTimestamp(left.timestamp)
    const rightTs = toComparableTimestamp(right.timestamp)

    if (leftTs < rightTs) {
      result.push(left)
      i++
    } else if (leftTs > rightTs) {
      result.push(right)
      j++
    } else {
      const leftKey = getLogIdentityKey(left)
      const rightKey = getLogIdentityKey(right)
      const cmp = leftKey.localeCompare(rightKey)
      if (cmp < 0) {
        result.push(left)
        i++
      } else if (cmp > 0) {
        result.push(right)
        j++
      } else {
        result.push(right)
        i++
        j++
      }
    }
  }

  while (i < existingItems.length) {
    result.push(existingItems[i]!)
    i++
  }
  while (j < sortedNext.length) {
    result.push(sortedNext[j]!)
    j++
  }

  return result
}

export function canAppendInPlace(items: LogSummary[], log: LogSummary): boolean {
  if (items.length === 0) {
    return true
  }
  const last = items[items.length - 1]!
  const lastTs = toComparableTimestamp(last.timestamp)
  const logTs = toComparableTimestamp(log.timestamp)
  if (logTs > lastTs) {
    return true
  }
  if (logTs < lastTs) {
    return false
  }
  return getLogIdentityKey(log).localeCompare(getLogIdentityKey(last)) >= 0
}

export function normalizeLogListResponseItems(response: LogListResponse | null | undefined) {
  return sortLogItemsAsc(response?.items ?? [])
}

export function getLogIdentityKey(log: LogSummary) {
  if (log.log_id) {
    return log.log_id
  }
  return [
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

export function normalizeFilterValues(values: string[] | undefined | null, single?: string | null) {
  const normalized: string[] = []
  const seen = new Set<string>()
  for (const value of [single ?? '', ...(values ?? [])]) {
    const nextValue = normalizeFilterValue(value)
    if (!nextValue || seen.has(nextValue)) {
      continue
    }
    seen.add(nextValue)
    normalized.push(nextValue)
  }
  return normalized
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
