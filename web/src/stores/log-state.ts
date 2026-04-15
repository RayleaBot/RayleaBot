import { computed, ref } from 'vue'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type { LogListResponse, LogPage, LogPageDirection, LogProtocol, LogSummary } from '@/types/api'

export const DEFAULT_LOG_PAGE_LIMIT = 50
export const MAX_LOG_PAGE_LIMIT = 200
export const LOG_PAGE_SIZE_OPTIONS = [50, 100, 200] as const

export interface LogFilters {
  level?: string
  source?: string
  protocol?: LogProtocol
  pluginId?: string
  requestId?: string
  limit?: number
}

interface FixedLogFilters {
  protocol?: LogProtocol
}

interface LogPageRequest {
  cursor?: string | null
  direction?: LogPageDirection
}

interface FetchPageOptions {
  preserveVisibleCurrent?: boolean
}

export function createLogsState(initialFilters: LogFilters = {}, fixedFilters: FixedLogFilters = {}) {
  const defaultLimit = normalizeLogLimit(initialFilters.limit)
  const items = ref<LogSummary[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const active = ref(true)
  const filters = ref<LogFilters>({
    ...initialFilters,
    limit: defaultLimit,
  })
  const page = ref<LogPage>(createEmptyPage(defaultLimit))
  const pendingNewCount = ref(0)
  const isLatestPage = ref(true)
  const livePageDirty = ref(false)
  const liveOverlay = ref<LogSummary[]>([])
  const needsActivationRefresh = ref(false)
  const canLoadOlder = computed(() => (
    page.value.has_older || (isLatestPage.value && livePageDirty.value && items.value.length > 0)
  ))
  const canLoadNewer = computed(() => (
    !isLatestPage.value && (page.value.has_newer || pendingNewCount.value > 0)
  ))
  const needsLatestRefresh = computed(() => (
    isLatestPage.value && needsActivationRefresh.value
  ))

  let requestVersion = 0

  async function fetchList() {
    return fetchPage()
  }

  async function goToLatestPage() {
    return fetchPage()
  }

  async function restoreLatestPage() {
    return fetchPage({}, {
      preserveVisibleCurrent: true,
    })
  }

  async function goToOlderPage() {
    if (loading.value) {
      return items.value
    }

    if (isLatestPage.value && livePageDirty.value) {
      await fetchPage()
    }

    const olderCursor = page.value.older_cursor
    if (!olderCursor) {
      return items.value
    }

    return fetchPage({
      cursor: olderCursor,
      direction: 'older',
    })
  }

  async function goToNewerPage() {
    if (loading.value) {
      return items.value
    }

    const newerCursor = page.value.newer_cursor
    if (newerCursor) {
      return fetchPage({
        cursor: newerCursor,
        direction: 'newer',
      })
    }

    if (!isLatestPage.value && pendingNewCount.value > 0) {
      return fetchPage()
    }

    return items.value
  }

  function append(log: LogSummary) {
    if (!matchesLogFilters(log, filters.value, fixedFilters)) {
      return false
    }

    const limit = normalizeLogLimit(filters.value.limit)
    liveOverlay.value = dedupeLogItems([log, ...liveOverlay.value], limit)

    if (!isLatestPage.value) {
      pendingNewCount.value += 1
      return true
    }

    if (!active.value) {
      livePageDirty.value = true
      needsActivationRefresh.value = true
      return true
    }

    const previousLength = items.value.length
    const nextItems = dedupeLogItems([log, ...items.value], limit)
    const truncatedLatestPage = previousLength >= limit && nextItems.length === limit

    items.value = nextItems
    page.value = {
      ...page.value,
      limit,
      has_newer: false,
      has_older: page.value.has_older || truncatedLatestPage,
    }
    pendingNewCount.value = 0
    livePageDirty.value = true
    return true
  }

  async function fetchPage(request: LogPageRequest = {}, options: FetchPageOptions = {}) {
    loading.value = true
    error.value = null
    requestVersion += 1
    const currentVersion = requestVersion

    try {
      const response = await apiRequest<LogListResponse>(buildLogListPath(filters.value, request, fixedFilters))
      if (currentVersion !== requestVersion) {
        return items.value
      }

      const limit = normalizeLogLimit(filters.value.limit)
      const freshItems = dedupeLogItems(response.items ?? [], limit)
      const isLatestRequest = !request.cursor
      const filteredOverlay = dedupeLogItems(
        liveOverlay.value.filter((item) => matchesLogFilters(item, filters.value, fixedFilters)),
        limit,
      )
      const preservedVisibleItems = options.preserveVisibleCurrent
        ? dedupeLogItems(
          items.value.filter((item) => matchesLogFilters(item, filters.value, fixedFilters)),
          limit,
        )
        : []

      items.value = isLatestRequest
        ? mergeLatestLogItems(freshItems, filteredOverlay, preservedVisibleItems, limit)
        : freshItems
      page.value = normalizePageInfo(response.page, limit)
      liveOverlay.value = isLatestRequest
        ? pruneResolvedOverlay(filteredOverlay, freshItems, limit)
        : filteredOverlay
      pendingNewCount.value = 0
      isLatestPage.value = !page.value.has_newer
      livePageDirty.value = false
      needsActivationRefresh.value = false
      return items.value
    } catch (err) {
      if (currentVersion === requestVersion) {
        error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      }
      throw err
    } finally {
      if (currentVersion === requestVersion) {
        loading.value = false
      }
    }
  }

  return {
    activate() {
      active.value = true
    },
    deactivate() {
      active.value = false
    },
    canLoadNewer,
    canLoadOlder,
    error,
    filters,
    isLatestPage,
    items,
    loading,
    needsLatestRefresh,
    page,
    pendingNewCount,
    append,
    fetchList,
    goToLatestPage,
    restoreLatestPage,
    goToNewerPage,
    goToOlderPage,
  }
}

export function buildLogListPath(
  filters: LogFilters,
  pageRequest: LogPageRequest = {},
  fixedFilters: FixedLogFilters = {},
) {
  const params = new URLSearchParams()
  const level = normalizeFilterValue(filters.level)
  const source = normalizeFilterValue(filters.source)
  const protocol = fixedFilters.protocol ?? filters.protocol
  const pluginId = normalizeFilterValue(filters.pluginId)
  const requestId = normalizeFilterValue(filters.requestId)
  const cursor = normalizeFilterValue(pageRequest.cursor)

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
  if (cursor) {
    params.set('cursor', cursor)
  }
  if (pageRequest.direction) {
    params.set('direction', pageRequest.direction)
  }
  params.set('limit', String(normalizeLogLimit(filters.limit)))

  return `/api/logs?${params.toString()}`
}

export function matchesLogFilters(log: LogSummary, filters: LogFilters, fixedFilters: FixedLogFilters = {}) {
  if (filters.level && log.level !== filters.level) {
    return false
  }
  if (filters.source && log.source !== filters.source) {
    return false
  }

  const effectiveProtocol = fixedFilters.protocol ?? filters.protocol
  if (effectiveProtocol && log.protocol !== effectiveProtocol) {
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
  const normalizedFallback = (
    Number.isFinite(fallback) && fallback >= 1
      ? Math.floor(fallback)
      : DEFAULT_LOG_PAGE_LIMIT
  )
  if (!limit || !Number.isFinite(limit) || limit < 1) {
    return Math.min(MAX_LOG_PAGE_LIMIT, normalizedFallback)
  }
  return Math.min(MAX_LOG_PAGE_LIMIT, Math.floor(limit))
}

export function dedupeLogItems(logs: LogSummary[], limit: number) {
  const normalizedLimit = normalizeLogLimit(limit)
  const merged: LogSummary[] = []
  const seen = new Set<string>()

  for (const log of logs) {
    const key = getLogIdentityKey(log)
    if (seen.has(key)) {
      continue
    }
    seen.add(key)
    merged.push(log)
    if (merged.length >= normalizedLimit) {
      break
    }
  }

  return merged
}

function mergeLatestLogItems(
  freshItems: LogSummary[],
  overlayItems: LogSummary[],
  visibleItems: LogSummary[],
  limit: number,
) {
  const normalizedLimit = normalizeLogLimit(limit)
  const merged = [...freshItems, ...overlayItems, ...visibleItems].map((log, index) => ({
    index,
    key: getLogIdentityKey(log),
    log,
    timestamp: toComparableTimestamp(log.timestamp),
  }))
  const deduped = new Map<string, (typeof merged)[number]>()

  for (const entry of merged) {
    if (!deduped.has(entry.key)) {
      deduped.set(entry.key, entry)
    }
  }

  return Array.from(deduped.values())
    .sort((left, right) => {
      if (left.timestamp !== right.timestamp) {
        return right.timestamp - left.timestamp
      }
      return left.index - right.index
    })
    .slice(0, normalizedLimit)
    .map((entry) => entry.log)
}

function pruneResolvedOverlay(overlayItems: LogSummary[], freshItems: LogSummary[], limit: number) {
  const freshKeys = new Set(freshItems.map((item) => getLogIdentityKey(item)))
  return dedupeLogItems(overlayItems.filter((item) => !freshKeys.has(getLogIdentityKey(item))), limit)
}

function createEmptyPage(limit: number): LogPage {
  return {
    limit,
    has_older: false,
    has_newer: false,
    older_cursor: null,
    newer_cursor: null,
  }
}

function normalizePageInfo(page: LogPage | undefined, limit: number): LogPage {
  if (!page) {
    return createEmptyPage(limit)
  }

  return {
    limit: normalizeLogLimit(page.limit, limit),
    has_older: Boolean(page.has_older),
    has_newer: Boolean(page.has_newer),
    older_cursor: page.older_cursor ?? null,
    newer_cursor: page.newer_cursor ?? null,
  }
}

function normalizeFilterValue(value: string | undefined | null) {
  const nextValue = value?.trim()
  return nextValue ? nextValue : ''
}

function getLogIdentityKey(log: LogSummary) {
  return log.log_id || [
    log.timestamp,
    log.level,
    log.protocol ?? '',
    log.source,
    log.plugin_id ?? '',
    log.request_id ?? '',
    log.message,
  ].join('|')
}

function toComparableTimestamp(value: string) {
  const numeric = Number(value)
  if (Number.isFinite(numeric) && numeric > 0) {
    return numeric >= 1_000_000_000_000 ? numeric : numeric * 1000
  }

  const parsed = Date.parse(value)
  if (Number.isFinite(parsed) && parsed > 0) {
    return parsed
  }

  return 0
}
