import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import {
  buildLogListPath,
  mergeLogItemsAsc,
  normalizeLogLimit,
  normalizeLogListResponseItems,
  type HistoryTimeRange,
  type LogFilters,
} from '@/stores/log-state'
import type { LogListResponse, LogSummary } from '@/types/api'

const historyPageLimit = 100

export interface HistoryTimeRangeInput {
  startLocal: string
  endLocal: string
}

export const useLogHistoryStore = defineStore('log-history', () => {
  const items = ref<LogSummary[]>([])
  const filters = ref<LogFilters>({})
  const timeRangeInput = ref<HistoryTimeRangeInput>({
    startLocal: '',
    endLocal: '',
  })
  const customTimeRange = ref(false)
  const anchorAt = ref('')
  const loading = ref(false)
  const loadingOlder = ref(false)
  const error = ref<string | null>(null)
  const olderCursor = ref<string | null>(null)
  const hasOlder = ref(false)
  const initialized = ref(false)

  let requestVersion = 0

  const pageLimit = computed(() => normalizeLogLimit(historyPageLimit, historyPageLimit))

  async function refreshAnchor() {
    anchorAt.value = new Date().toISOString()
    if (!customTimeRange.value) {
      const anchorDate = new Date(anchorAt.value)
      const startDate = new Date(anchorDate.getTime() - 24 * 60 * 60 * 1000)
      timeRangeInput.value = {
        startLocal: toLocalDateTimeInput(startDate),
        endLocal: toLocalDateTimeInput(anchorDate),
      }
    }

    items.value = []
    olderCursor.value = null
    hasOlder.value = false
    initialized.value = false
    return fetchLatest()
  }

  async function applyFilters() {
    customTimeRange.value = true
    items.value = []
    olderCursor.value = null
    hasOlder.value = false
    initialized.value = false
    return fetchLatest()
  }

  function resetTimeRangeToDefault() {
    customTimeRange.value = false
  }

  async function loadOlder() {
    if (!olderCursor.value || loadingOlder.value) {
      return items.value
    }

    loadingOlder.value = true
    error.value = null

    try {
      const response = await apiRequest<LogListResponse>(buildLogListPath({
        scope: 'history',
        filters: filters.value,
        timeRange: currentUtcRange(),
        cursor: olderCursor.value,
        direction: 'older',
        limit: pageLimit.value,
      }))

      items.value = mergeLogItemsAsc(items.value, normalizeLogListResponseItems(response))
      olderCursor.value = response.page?.older_cursor ?? null
      hasOlder.value = Boolean(response.page?.has_older)
      initialized.value = true
      return items.value
    } catch (err) {
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      loadingOlder.value = false
    }
  }

  async function fetchLatest() {
    loading.value = true
    error.value = null
    requestVersion += 1
    const currentVersion = requestVersion

    try {
      const response = await apiRequest<LogListResponse>(buildLogListPath({
        scope: 'history',
        filters: filters.value,
        timeRange: currentUtcRange(),
        limit: pageLimit.value,
      }))
      if (currentVersion !== requestVersion) {
        return items.value
      }

      items.value = normalizeLogListResponseItems(response)
      olderCursor.value = response.page?.older_cursor ?? null
      hasOlder.value = Boolean(response.page?.has_older)
      initialized.value = true
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

  function currentUtcRange(): HistoryTimeRange {
    return {
      startAt: localDateTimeToUtc(timeRangeInput.value.startLocal),
      endAt: localDateTimeToUtc(timeRangeInput.value.endLocal),
    }
  }

  return {
    anchorAt,
    customTimeRange,
    error,
    filters,
    hasOlder,
    initialized,
    items,
    loading,
    loadingOlder,
    timeRangeInput,
    applyFilters,
    currentUtcRange,
    loadOlder,
    refreshAnchor,
    resetTimeRangeToDefault,
  }
})

export function toLocalDateTimeInput(value: Date) {
  const year = value.getFullYear()
  const month = String(value.getMonth() + 1).padStart(2, '0')
  const day = String(value.getDate()).padStart(2, '0')
  const hours = String(value.getHours()).padStart(2, '0')
  const minutes = String(value.getMinutes()).padStart(2, '0')
  return `${year}-${month}-${day}T${hours}:${minutes}`
}

export function localDateTimeToUtc(value: string) {
  const trimmed = value.trim()
  if (!trimmed) {
    return ''
  }

  const parsed = new Date(trimmed)
  if (Number.isNaN(parsed.getTime())) {
    return ''
  }

  return parsed.toISOString().replace(/\.\d{3}Z$/, 'Z')
}
