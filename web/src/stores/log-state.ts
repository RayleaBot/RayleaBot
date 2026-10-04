import { toRaw } from 'vue'
import { timestampMilliseconds } from '@/lib/timestamp'
import type { LogLevel, LogListResponse, LogPageDirection, LogProtocol, LogSummary } from '@/types/api'

export type LogScope = 'history' | 'current_session'

const DEFAULT_LOG_PAGE_LIMIT = 100
const MAX_LOG_PAGE_LIMIT = 200

export interface LogFilters {
  levels?: LogLevel[]
  source?: string
  protocol?: LogProtocol
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

export function buildLogListPath(options: BuildLogListPathOptions): `/api/logs?${string}` {
  const params = new URLSearchParams()
  const filters = options.filters ?? {}

  params.set('scope', options.scope)
  params.set('limit', String(normalizeLogLimit(options.limit)))

  const levels = normalizeFilterValues(filters.levels)
  const source = normalizeFilterValue(filters.source)
  const protocol = normalizeFilterValue(filters.protocol)
  const pluginIds = normalizeFilterValues(filters.pluginIds)
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
  return createLogFilterMatcher(filters)(log)
}

export function createLogFilterMatcher(filters: LogFilters) {
  const levels = normalizeFilterValues(filters.levels)
  const pluginIds = normalizeFilterValues(filters.pluginIds)

  return (log: LogSummary) => {
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

const timestampCache = new WeakMap<LogSummary, { source: string; value: bigint }>()

function logTimestamp(log: LogSummary) {
  const raw = toRaw(log)
  const cached = timestampCache.get(raw)
  if (cached?.source === raw.timestamp) return cached.value
  const value = toComparableTimestamp(raw.timestamp)
  timestampCache.set(raw, { source: raw.timestamp, value })
  return value
}

export function compareLogTimestamps(left: LogSummary, right: LogSummary) {
  const leftTimestamp = logTimestamp(left)
  const rightTimestamp = logTimestamp(right)
  return leftTimestamp === rightTimestamp ? 0 : leftTimestamp < rightTimestamp ? -1 : 1
}

export function compareLogItems(left: LogSummary, right: LogSummary) {
  const timestampOrder = compareLogTimestamps(left, right)
  if (timestampOrder !== 0) return timestampOrder
  return getLogIdentityKey(left).localeCompare(getLogIdentityKey(right))
}

export function sortLogItemsAsc(items: LogSummary[]) {
  return [...toRaw(items)].sort(compareLogItems)
}

export function mergeSortedLogItemsAsc(existingItems: LogSummary[], nextItems: LogSummary[]) {
  const existing = toRaw(existingItems)
  const nextMap = new Map<string, LogSummary>()
  for (const item of toRaw(nextItems)) nextMap.set(getLogIdentityKey(item), item)
  const sortedNext = sortLogItemsAsc(Array.from(nextMap.values()))
  if (existing.length === 0) return sortedNext
  if (sortedNext.length === 0) return [...existing]

  const result: LogSummary[] = []
  let i = 0
  let j = 0
  while (i < existing.length) {
    const left = existing[i++]!
    // A repeated ID replaces the row even if its timestamp changed.
    if (nextMap.has(getLogIdentityKey(left))) continue
    while (j < sortedNext.length && compareLogItems(sortedNext[j]!, left) < 0) {
      result.push(sortedNext[j++]!)
    }
    result.push(left)
  }
  while (j < sortedNext.length) result.push(sortedNext[j++]!)
  return result
}

export function canAppendInPlace(items: LogSummary[], log: LogSummary): boolean {
  const raw = toRaw(items)
  return raw.length === 0 || compareLogItems(raw[raw.length - 1]!, log) < 0
}

export function normalizeLogListResponseItems(response: LogListResponse | null | undefined) {
  return mergeSortedLogItemsAsc([], response?.items ?? [])
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

export function normalizeFilterValues(values: string[] | undefined | null) {
  const normalized: string[] = []
  const seen = new Set<string>()
  for (const value of (values ?? [])) {
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
  const milliseconds = timestampMilliseconds(value)
  if (milliseconds === null) return 0n
  // Date 只保留毫秒；日志排序还需要 RFC3339 时间戳的小数余量。
  const fraction = value.trim().match(/\.(\d{1,9})(?:Z|[+-]\d{2}:\d{2})$/i)?.[1]
  const remainder = fraction ? BigInt(fraction.padEnd(9, '0').slice(3)) : 0n
  return BigInt(Math.trunc(milliseconds)) * 1_000_000n + remainder
}

function sameFilterValues(left: string[], right: string[]) {
  const normalizedLeft = [...left].sort((a, b) => a.localeCompare(b, 'zh-CN'))
  const normalizedRight = [...right].sort((a, b) => a.localeCompare(b, 'zh-CN'))
  return normalizedLeft.length === normalizedRight.length
    && normalizedLeft.every((item, index) => item === normalizedRight[index])
}

// Logs written outside any request carry this reserved request ID (see the log request_id contract).
// It correlates nothing, so it is neither shown on a row nor offered as a filter for related logs.
const SYSTEM_LOG_REQUEST_ID = 'system'

export function correlatedRequestId(requestId?: string | null) {
  return requestId && requestId !== SYSTEM_LOG_REQUEST_ID ? requestId : undefined
}

export function sameTimeRange(left: HistoryTimeRange, right: HistoryTimeRange) {
  return (left.startAt ?? '') === (right.startAt ?? '') && (left.endAt ?? '') === (right.endAt ?? '')
}

export function sameLogFilters(left: LogFilters, right: LogFilters) {
  return sameFilterValues(normalizeFilterValues(left.levels), normalizeFilterValues(right.levels))
    && (left.source ?? '') === (right.source ?? '')
    && (left.protocol ?? '') === (right.protocol ?? '')
    && sameFilterValues(normalizeFilterValues(left.pluginIds), normalizeFilterValues(right.pluginIds))
    && (left.requestId ?? '') === (right.requestId ?? '')
}
