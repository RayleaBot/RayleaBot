import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import { ONEBOT11_PROTOCOL } from '@/lib/protocols'
import {
  buildLogListPath,
  createLogsState,
  normalizeLogLimit,
  type LogFilters,
} from '@/stores/log-state'
import type { LogDetailResponse, LogSummary } from '@/types/api'

const protocolDetailCacheLimit = 64

export interface ProtocolLogFilters extends Omit<LogFilters, 'protocol'> {}

export const useProtocolLogsStore = defineStore('protocolLogs', () => {
  const base = createLogsState({
    limit: 200,
  }, {
    protocol: ONEBOT11_PROTOCOL,
  })

  const detailLoading = ref(false)
  const detailError = ref<string | null>(null)
  const active = ref(false)
  const selectedLogId = ref<string | null>(null)
  const currentDetail = ref<LogDetailResponse | null>(null)
  const detailCache = new Map<string, LogDetailResponse>()
  let detailRequestVersion = 0

  const selectedItem = computed(() => (
    selectedLogId.value
      ? base.items.value.find((item) => item.log_id === selectedLogId.value) ?? null
      : null
  ))

  async function fetchList() {
    const result = await base.fetchList()
    await ensureSelectionAfterPageChange()
    return result
  }

  async function goToLatestPage() {
    const result = await base.goToLatestPage()
    await ensureSelectionAfterPageChange()
    return result
  }

  async function restoreLatestPage() {
    const result = await base.restoreLatestPage()
    await ensureSelectionAfterPageChange()
    return result
  }

  async function goToOlderPage() {
    const result = await base.goToOlderPage()
    await ensureSelectionAfterPageChange()
    return result
  }

  async function goToNewerPage() {
    const result = await base.goToNewerPage()
    await ensureSelectionAfterPageChange()
    return result
  }

  async function selectLog(logId: string, options: { preferCache?: boolean } = {}) {
    const nextLogId = normalizeFilterValue(logId)
    if (!nextLogId) {
      selectedLogId.value = null
      currentDetail.value = null
      detailError.value = null
      return null
    }

    selectedLogId.value = nextLogId
    detailError.value = null

    const cached = getCachedDetail(nextLogId)
    if (cached && options.preferCache !== false) {
      currentDetail.value = cached
      return cached
    }

    if (!active.value) {
      currentDetail.value = cached ?? null
      return cached ?? null
    }

    detailLoading.value = true
    detailRequestVersion += 1
    const requestVersion = detailRequestVersion

    try {
      const response = await apiRequest<LogDetailResponse>(`/api/logs/${encodeURIComponent(nextLogId)}`)
      if (requestVersion !== detailRequestVersion || selectedLogId.value !== nextLogId) {
        return response
      }

      setCachedDetail(nextLogId, response)
      currentDetail.value = response
      return response
    } catch (err) {
      if (requestVersion === detailRequestVersion && selectedLogId.value === nextLogId) {
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
    const accepted = base.append(log)
    if (!accepted) {
      return false
    }

    if (!active.value) {
      return true
    }

    if (base.isLatestPage.value) {
      try {
        await selectLog(log.log_id, { preferCache: false })
      } catch {
        // detailError exposes the failure on the page
      }
      return true
    }

    if (selectedLogId.value === log.log_id) {
      currentDetail.value = getCachedDetail(log.log_id) ?? null
    }
    return true
  }

  function activate() {
    active.value = true
    base.activate()
  }

  function deactivate() {
    active.value = false
    base.deactivate()
  }

  async function ensureSelectionAfterPageChange() {
    if (base.items.value.length === 0) {
      selectedLogId.value = null
      currentDetail.value = null
      detailError.value = null
      return
    }

    const visibleSelection = selectedLogId.value
      ? base.items.value.find((item) => item.log_id === selectedLogId.value)
      : null
    const nextSelection = base.isLatestPage.value
      ? base.items.value[0]
      : (visibleSelection ?? base.items.value[0])

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

  function getCachedDetail(logId: string) {
    const cached = detailCache.get(logId)
    if (!cached) {
      return null
    }

    detailCache.delete(logId)
    detailCache.set(logId, cached)
    return cached
  }

  function setCachedDetail(logId: string, detail: LogDetailResponse) {
    if (detailCache.has(logId)) {
      detailCache.delete(logId)
    }
    detailCache.set(logId, detail)

    while (detailCache.size > protocolDetailCacheLimit) {
      const oldestKey = detailCache.keys().next().value
      if (!oldestKey) {
        break
      }
      detailCache.delete(oldestKey)
    }
  }

  return {
    active,
    canLoadNewer: base.canLoadNewer,
    canLoadOlder: base.canLoadOlder,
    currentDetail,
    detailError,
    detailLoading,
    error: base.error,
    filters: base.filters,
    isLatestPage: base.isLatestPage,
    items: base.items,
    loading: base.loading,
    needsLatestRefresh: base.needsLatestRefresh,
    page: base.page,
    pendingNewCount: base.pendingNewCount,
    selectedItem,
    selectedLogId,
    activate,
    appendLive,
    deactivate,
    fetchList,
    goToLatestPage,
    restoreLatestPage,
    goToNewerPage,
    goToOlderPage,
    selectLog,
  }
})

export function buildProtocolLogListPath(filters: ProtocolLogFilters, pageRequest?: {
  cursor?: string | null
  direction?: 'older' | 'newer'
}) {
  return buildLogListPath({
    ...filters,
    limit: normalizeLogLimit(filters.limit, 200),
  }, pageRequest, {
    protocol: ONEBOT11_PROTOCOL,
  })
}

function normalizeFilterValue(value: string | undefined | null) {
  const nextValue = value?.trim()
  return nextValue ? nextValue : ''
}
