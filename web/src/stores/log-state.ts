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
  const levels = normalizeFilterValues(filters.levels)
  const pluginIds = normalizeFilterValues(filters.pluginIds)

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
      return leftTimestamp < rightTimestamp ? -1 : 1
    }
    return getLogIdentityKey(left).localeCompare(getLogIdentityKey(right))
  })
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

function getLogIdentityKey(log: LogSummary) {
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
