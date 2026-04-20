import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import {
  buildLogListPath,
  canAppendInPlace,
  mergeSortedLogItemsAsc,
  normalizeLogLimit,
  normalizeLogListResponseItems,
  matchesLogFilters,
  type LogFilters,
} from '@/stores/log-state'
import type { LogListResponse, LogSummary } from '@/types/api'

const currentSessionPageLimit = 100
const MAX_LIVE_LOG_ITEMS = 5000

export const useLogsStore = defineStore('logs', () => {
  const items = ref<LogSummary[]>([])
  const filters = ref<LogFilters>({})
  const loading = ref(false)
  const loadingOlder = ref(false)
  const error = ref<string | null>(null)
  const olderCursor = ref<string | null>(null)
  const hasOlder = ref(false)
  const pendingNewCount = ref(0)
  const initialized = ref(false)
  const active = ref(false)
  const atBottom = ref(true)

  let requestVersion = 0
  let olderWindowExpanded = false

  const pageLimit = computed(() => normalizeLogLimit(currentSessionPageLimit, currentSessionPageLimit))

  async function ensureLoaded() {
    if (initialized.value || loading.value) {
      return items.value
    }
    return fetchLatest({ replaceItems: false })
  }

  async function applyFilters() {
    items.value = []
    olderCursor.value = null
    hasOlder.value = false
    pendingNewCount.value = 0
    initialized.value = false
    olderWindowExpanded = false
    return fetchLatest({ replaceItems: true })
  }

  async function refreshLatest() {
    return fetchLatest({ replaceItems: false })
  }

  async function loadOlder() {
    if (!olderCursor.value || loadingOlder.value) {
      return items.value
    }

    loadingOlder.value = true
    error.value = null

    try {
      const response = await apiRequest<LogListResponse>(buildLogListPath({
        scope: 'current_session',
        filters: filters.value,
        cursor: olderCursor.value,
        direction: 'older',
        limit: pageLimit.value,
      }))

      const olderItems = normalizeLogListResponseItems(response)
      items.value = mergeSortedLogItemsAsc(items.value, olderItems)
      olderCursor.value = response.page?.older_cursor ?? null
      hasOlder.value = Boolean(response.page?.has_older)
      initialized.value = true
      if (olderItems.length > 0) {
        olderWindowExpanded = true
      }
      return items.value
    } catch (err) {
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      loadingOlder.value = false
    }
  }

  function trimItems(value: LogSummary[]): LogSummary[] {
    if (olderWindowExpanded) {
      return value
    }
    if (value.length <= MAX_LIVE_LOG_ITEMS) {
      return value
    }
    return value.slice(value.length - MAX_LIVE_LOG_ITEMS)
  }

  function append(log: LogSummary) {
    if (!matchesLogFilters(log, filters.value)) {
      return false
    }

    if (canAppendInPlace(items.value, log)) {
      items.value = trimItems([...items.value, log])
    } else {
      items.value = trimItems(mergeSortedLogItemsAsc(items.value, [log]))
    }
    initialized.value = true

    if (active.value && atBottom.value) {
      pendingNewCount.value = 0
    } else {
      pendingNewCount.value += 1
    }

    return true
  }

  function appendBatch(logs: LogSummary[]) {
    const matching = logs.filter((log) => matchesLogFilters(log, filters.value))
    if (matching.length === 0) {
      return 0
    }

    items.value = trimItems(mergeSortedLogItemsAsc(items.value, matching))
    initialized.value = true

    if (active.value && atBottom.value) {
      pendingNewCount.value = 0
    } else {
      pendingNewCount.value += matching.length
    }

    return matching.length
  }

  function setViewportActive(nextValue: boolean) {
    active.value = nextValue
  }

  function setViewportAtBottom(nextValue: boolean) {
    atBottom.value = nextValue
    if (nextValue) {
      pendingNewCount.value = 0
    }
  }

  function acknowledgePendingNew() {
    pendingNewCount.value = 0
  }

  async function fetchLatest(options: { replaceItems: boolean }) {
    loading.value = true
    error.value = null
    requestVersion += 1
    const currentVersion = requestVersion

    try {
      const response = await apiRequest<LogListResponse>(buildLogListPath({
        scope: 'current_session',
        filters: filters.value,
        limit: pageLimit.value,
      }))
      if (currentVersion !== requestVersion) {
        return items.value
      }

      const nextItems = normalizeLogListResponseItems(response)
      items.value = options.replaceItems ? nextItems : mergeSortedLogItemsAsc(items.value, nextItems)
      if (options.replaceItems || !olderCursor.value) {
        olderCursor.value = response.page?.older_cursor ?? null
      }
      hasOlder.value = options.replaceItems
        ? Boolean(response.page?.has_older)
        : (hasOlder.value || Boolean(response.page?.has_older))
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

  return {
    active,
    atBottom,
    error,
    filters,
    hasOlder,
    initialized,
    items,
    loading,
    loadingOlder,
    pendingNewCount,
    acknowledgePendingNew,
    append,
    appendBatch,
    applyFilters,
    ensureLoaded,
    loadOlder,
    refreshLatest,
    setViewportActive,
    setViewportAtBottom,
  }
})
