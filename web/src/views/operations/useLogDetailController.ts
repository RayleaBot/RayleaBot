import { computed, ref } from 'vue'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type { LogDetailResponse, LogSummary } from '@/types/api'

const detailCacheLimit = 64

export function useLogDetailController() {
  const open = ref(false)
  const selectedSummary = ref<LogSummary | null>(null)
  const currentDetail = ref<LogDetailResponse | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  const cache = new Map<string, LogDetailResponse>()
  let requestVersion = 0

  const selectedLogId = computed(() => selectedSummary.value?.log_id ?? null)

  async function openDetail(summary: LogSummary) {
    const nextLogId = summary.log_id?.trim()
    if (!nextLogId) {
      return null
    }

    open.value = true
    selectedSummary.value = summary
    error.value = null

    const cached = getCachedDetail(nextLogId)
    currentDetail.value = cached ?? null
    if (cached) {
      return cached
    }

    loading.value = true
    requestVersion += 1
    const currentVersion = requestVersion

    try {
      const detail = await apiRequest<LogDetailResponse>(`/api/logs/${encodeURIComponent(nextLogId)}`)
      if (currentVersion !== requestVersion || selectedLogId.value !== nextLogId) {
        return detail
      }

      setCachedDetail(nextLogId, detail)
      currentDetail.value = detail
      return detail
    } catch (err) {
      if (currentVersion === requestVersion && selectedLogId.value === nextLogId) {
        error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      }
      throw err
    } finally {
      if (currentVersion === requestVersion) {
        loading.value = false
      }
    }
  }

  function closeDetail() {
    open.value = false
    loading.value = false
    error.value = null
    currentDetail.value = null
    selectedSummary.value = null
  }

  function getCachedDetail(logId: string) {
    const cached = cache.get(logId)
    if (!cached) {
      return null
    }

    cache.delete(logId)
    cache.set(logId, cached)
    return cached
  }

  function setCachedDetail(logId: string, detail: LogDetailResponse) {
    if (cache.has(logId)) {
      cache.delete(logId)
    }
    cache.set(logId, detail)

    while (cache.size > detailCacheLimit) {
      const oldestKey = cache.keys().next().value
      if (!oldestKey) {
        break
      }
      cache.delete(oldestKey)
    }
  }

  return {
    currentDetail,
    error,
    loading,
    open,
    selectedLogId,
    selectedSummary,
    closeDetail,
    openDetail,
  }
}
