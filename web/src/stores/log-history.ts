import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import { t } from '@/i18n'
import { managementTimeZone } from '@/lib/format'
import { timeZoneDateTimeToUtc, toTimeZoneDateTimeInput } from '@/lib/time-zone'
import {
  buildLogListPath,
  mergeSortedLogItemsAsc,
  normalizeLogLimit,
  normalizeLogListResponseItems,
  sameTimeRange,
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
  // The range of the loaded list. The inputs above are a draft until a query is issued, so paging older
  // entries keeps using the range the list was loaded with.
  const appliedRange = ref<HistoryTimeRange>({})
  // The quick range the loaded list came from; a hand-edited range belongs to none of them.
  const recentDays = ref<number | null>(1)
  const anchorAt = ref('')
  const loading = ref(false)
  const loadingOlder = ref(false)
  const error = ref<string | null>(null)
  const olderCursor = ref<string | null>(null)
  const hasOlder = ref(false)
  const initialized = ref(false)

  let requestVersion = 0

  watch(managementTimeZone, (next, previous) => {
    for (const key of ['startLocal', 'endLocal'] as const) {
      const instant = timeZoneDateTimeToUtc(timeRangeInput.value[key], previous, key === 'endLocal' ? 'end' : 'start')
      if (instant) timeRangeInput.value[key] = toTimeZoneDateTimeInput(new Date(instant), next)
    }
  }, { flush: 'sync' })

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

  // Why the edited range cannot be queried, tied to the field to fix; null when it can.
  function timeRangeIssue(): { field: 'start' | 'end'; message: string } | null {
    const range = currentUtcRange()
    if (timeRangeInput.value.startLocal && !range.startAt) return { field: 'start', message: t('logs.history.invalidTimeZoneTime') }
    if (timeRangeInput.value.endLocal && !range.endAt) return { field: 'end', message: t('logs.history.invalidTimeZoneTime') }
    if (range.startAt && range.endAt && range.startAt > range.endAt) return { field: 'end', message: t('logs.history.invalidTimeRange') }
    return null
  }

  async function applyFilters() {
    const range = currentUtcRange()
    const issue = timeRangeIssue()
    if (issue) {
      error.value = issue.message
      throw new Error(error.value)
    }
    customTimeRange.value = true
    if (!sameTimeRange(range, appliedRange.value)) recentDays.value = null
    items.value = []
    olderCursor.value = null
    hasOlder.value = false
    initialized.value = false
    return fetchLatest()
  }

  function resetTimeRangeToDefault() {
    customTimeRange.value = false
    recentDays.value = 1
  }

  function setTimeRange(days: number) {
    const anchorDate = new Date()
    const startDate = new Date(anchorDate.getTime() - days * 24 * 60 * 60 * 1000)
    timeRangeInput.value = {
      startLocal: toLocalDateTimeInput(startDate),
      endLocal: toLocalDateTimeInput(anchorDate),
    }
    customTimeRange.value = true
    recentDays.value = days
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
        timeRange: appliedRange.value,
        cursor: olderCursor.value,
        direction: 'older',
        limit: pageLimit.value,
      }))

      items.value = mergeSortedLogItemsAsc(items.value, normalizeLogListResponseItems(response))
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
    appliedRange.value = currentUtcRange()

    try {
      const response = await apiRequest<LogListResponse>(buildLogListPath({
        scope: 'history',
        filters: filters.value,
        timeRange: appliedRange.value,
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
      endAt: localDateTimeToUtc(timeRangeInput.value.endLocal, 'end'),
    }
  }

  return {
    anchorAt,
    appliedRange,
    customTimeRange,
    recentDays,
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
    setTimeRange,
    timeRangeIssue,
  }
})

export function toLocalDateTimeInput(value: Date) {
  return toTimeZoneDateTimeInput(value, managementTimeZone())
}

export function localDateTimeToUtc(value: string, boundary: 'start' | 'end' = 'start') {
  return timeZoneDateTimeToUtc(value, managementTimeZone(), boundary)
}
