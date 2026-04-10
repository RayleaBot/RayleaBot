import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import { ONEBOT11_PROTOCOL } from '@/lib/protocols'
import type { LogDetailResponse, LogListResponse, LogSummary } from '@/types/api'

const protocolLogBufferLimit = 200

export interface ProtocolLogFilters {
  level?: string
  source?: string
  requestId?: string
  limit?: number
}

export const useProtocolLogsStore = defineStore('protocolLogs', () => {
  const items = ref<LogSummary[]>([])
  const loading = ref(false)
  const detailLoading = ref(false)
  const error = ref<string | null>(null)
  const detailError = ref<string | null>(null)
  const active = ref(false)
  const autoFollow = ref(true)
  const filters = ref<ProtocolLogFilters>({
    limit: protocolLogBufferLimit,
  })
  const selectedLogId = ref<string | null>(null)
  const currentDetail = ref<LogDetailResponse | null>(null)

  const detailCache = new Map<string, LogDetailResponse>()
  let detailRequestVersion = 0

  const selectedItem = computed(() => (
    selectedLogId.value
      ? items.value.find((item) => item.log_id === selectedLogId.value) ?? null
      : null
  ))

  async function fetchList() {
    loading.value = true
    error.value = null
    try {
      const response = await apiRequest<LogListResponse>(buildProtocolLogListPath(filters.value))
      items.value = mergeBuffer(response.items, items.value, filters.value.limit ?? protocolLogBufferLimit)
      await ensureSelectionAfterRefresh()
      return items.value
    } catch (err) {
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      loading.value = false
    }
  }

  async function selectLog(logId: string, options: { preferCache?: boolean } = {}) {
    const nextLogID = normalizeFilterValue(logId)
    if (!nextLogID) {
      selectedLogId.value = null
      currentDetail.value = null
      detailError.value = null
      return null
    }

    selectedLogId.value = nextLogID
    detailError.value = null

    const cached = detailCache.get(nextLogID)
    if (cached && options.preferCache !== false) {
      currentDetail.value = cached
      return cached
    }

    detailLoading.value = true
    detailRequestVersion += 1
    const requestVersion = detailRequestVersion

    try {
      const response = await apiRequest<LogDetailResponse>(`/api/logs/${encodeURIComponent(nextLogID)}`)
      if (requestVersion !== detailRequestVersion || selectedLogId.value !== nextLogID) {
        return response
      }

      detailCache.set(nextLogID, response)
      currentDetail.value = response
      return response
    } catch (err) {
      if (requestVersion === detailRequestVersion && selectedLogId.value === nextLogID) {
        detailError.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
        currentDetail.value = null
      }
      throw err
    } finally {
      if (requestVersion === detailRequestVersion) {
        detailLoading.value = false
      }
    }
  }

  async function appendLive(log: LogSummary) {
    if (!matchesRealtimeFilters(log, filters.value)) {
      return false
    }

    items.value = appendToBuffer(items.value, log, filters.value.limit ?? protocolLogBufferLimit)
    if (!active.value) {
      return true
    }

    if (autoFollow.value) {
      try {
        await selectLog(log.log_id, { preferCache: false })
      } catch {
        // detailError exposes the failure on the page
      }
      return true
    }

    if (selectedLogId.value === log.log_id) {
      const cached = detailCache.get(log.log_id)
      if (cached) {
        currentDetail.value = cached
      }
    }
    return true
  }

  function clearBuffer() {
    items.value = []
    selectedLogId.value = null
    currentDetail.value = null
    detailError.value = null
  }

  function activate() {
    active.value = true
  }

  function deactivate() {
    active.value = false
  }

  async function resumeAutoFollow() {
    autoFollow.value = true
    const latest = items.value.at(-1)
    if (!latest) {
      return
    }

    try {
      await selectLog(latest.log_id)
    } catch {
      // detailError exposes the failure on the page
    }
  }

  function pauseAutoFollow() {
    autoFollow.value = false
  }

  async function ensureSelectionAfterRefresh() {
    if (items.value.length === 0) {
      selectedLogId.value = null
      currentDetail.value = null
      detailError.value = null
      return
    }

    const visibleSelection = selectedLogId.value
      ? items.value.find((item) => item.log_id === selectedLogId.value)
      : null
    const fallback = items.value.at(-1) ?? null
    const nextSelection = autoFollow.value
      ? fallback
      : (visibleSelection ?? fallback)

    if (!nextSelection) {
      selectedLogId.value = null
      currentDetail.value = null
      detailError.value = null
      return
    }

    try {
      await selectLog(nextSelection.log_id)
    } catch {
      // detailError exposes the failure on the page
    }
  }

  return {
    active,
    autoFollow,
    currentDetail,
    detailError,
    detailLoading,
    error,
    filters,
    items,
    loading,
    selectedItem,
    selectedLogId,
    activate,
    appendLive,
    clearBuffer,
    deactivate,
    fetchList,
    pauseAutoFollow,
    resumeAutoFollow,
    selectLog,
  }
})

export function buildProtocolLogListPath(filters: ProtocolLogFilters) {
  const params = new URLSearchParams()
  params.set('protocol', ONEBOT11_PROTOCOL)
  params.set('limit', String(normalizeLimit(filters.limit)))

  const level = normalizeFilterValue(filters.level)
  const source = normalizeFilterValue(filters.source)
  const requestId = normalizeFilterValue(filters.requestId)

  if (level) {
    params.set('level', level)
  }
  if (source) {
    params.set('source', source)
  }
  if (requestId) {
    params.set('request_id', requestId)
  }

  return `/api/logs?${params.toString()}`
}

function normalizeBuffer(items: LogSummary[], limit: number) {
  return mergeBuffer(items, [], limit)
}

function appendToBuffer(existing: LogSummary[], incoming: LogSummary, limit: number) {
  return mergeBuffer(existing, [incoming], limit)
}

function mergeBuffer(primary: LogSummary[], secondary: LogSummary[], limit: number) {
  const nextItems = new Map<string, LogSummary>()
  for (const item of [...primary, ...secondary]) {
    if (nextItems.has(item.log_id)) {
      nextItems.delete(item.log_id)
    }
    nextItems.set(item.log_id, item)
  }
  return Array.from(nextItems.values()).slice(-normalizeLimit(limit))
}

function matchesRealtimeFilters(log: LogSummary, filters: ProtocolLogFilters) {
  if (log.protocol !== ONEBOT11_PROTOCOL) {
    return false
  }

  const level = normalizeFilterValue(filters.level)
  if (level && log.level !== level) {
    return false
  }

  const source = normalizeFilterValue(filters.source)
  if (source && log.source !== source) {
    return false
  }

  const requestId = normalizeFilterValue(filters.requestId)
  if (requestId && log.request_id !== requestId) {
    return false
  }

  return true
}

function normalizeFilterValue(value: string | undefined | null) {
  const nextValue = value?.trim()
  return nextValue ? nextValue : ''
}

function normalizeLimit(limit: number | undefined) {
  if (!limit || limit < 1) {
    return protocolLogBufferLimit
  }
  return Math.min(limit, protocolLogBufferLimit)
}
