import { onScopeDispose, ref, toRaw } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import {
  buildLogListPath,
  canAppendInPlace,
  compareLogItems,
  compareLogTimestamps,
  createLogFilterMatcher,
  getLogIdentityKey,
  mergeSortedLogItemsAsc,
  normalizeLogListResponseItems,
  type LogFilters,
} from '@/stores/log-state'
import type { LogListResponse, LogSummary } from '@/types/api'

const pageLimit = 100
const reconcileIntervalMs = 2000

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

  let version = 0
  let controller = new AbortController()
  let latestPromise: Promise<LogSummary[]> | undefined
  let needsReload = false
  let confirmedNewest: string | undefined
  let streamDuringRead: LogSummary[] | undefined
  let reconcileTimer: ReturnType<typeof setTimeout> | undefined
  let reconciling = false
  let dirty = false
  let indexedItems: LogSummary[] | undefined
  let indexedLength = 0
  let identities = new Set<string>()

  function resetRequests() {
    version += 1
    controller.abort()
    controller = new AbortController()
    if (reconcileTimer !== undefined) clearTimeout(reconcileTimer)
    reconcileTimer = undefined
    latestPromise = undefined
    loading.value = false
    loadingOlder.value = false
    reconciling = false
    streamDuringRead = undefined
  }

  function requestPage(signal: AbortSignal, cursor?: string | null) {
    return apiRequest<LogListResponse>(buildLogListPath({
      scope: 'current_session', filters: filters.value, limit: pageLimit,
      ...(cursor ? { cursor, direction: 'older' } : {}),
    }), { signal })
  }

  function ensureLoaded() {
    if (latestPromise) return latestPromise
    if (initialized.value && !needsReload) return Promise.resolve(items.value)
    return fetchWindow(needsReload ? Math.max(1, Math.ceil(items.value.length / pageLimit)) : 1, !needsReload)
  }

  function applyFilters() {
    items.value = []
    olderCursor.value = null
    hasOlder.value = false
    pendingNewCount.value = 0
    initialized.value = false
    return fetchWindow(1)
  }

  function fetchWindow(pages: number, preserveUnconfirmed = true) {
    const initialStream = !initialized.value && preserveUnconfirmed ? toRaw(items.value) : []
    resetRequests()
    const currentVersion = version
    const signal = controller.signal
    streamDuringRead = [...initialStream]
    loading.value = true
    error.value = null
    dirty = false
    latestPromise = (async () => {
      try {
        let response = await requestPage(signal)
        if (currentVersion !== version) return items.value
        let next = normalizeLogListResponseItems(response)
        const newest = next.at(-1)?.log_id
        for (let page = 1; page < pages && response.page?.has_older && response.page.older_cursor; page += 1) {
          await yieldPage(signal)
          response = await requestPage(signal, response.page.older_cursor)
          if (currentVersion !== version) return items.value
          next.push(...normalizeLogListResponseItems(response))
        }
        next = mergeSortedLogItemsAsc([], next)
        items.value = mergeWindowStream(next, streamDuringRead ?? [], Boolean(response.page?.has_older))
        olderCursor.value = response.page?.older_cursor ?? null
        hasOlder.value = Boolean(response.page?.has_older)
        confirmedNewest = newest
        initialized.value = true
        needsReload = false
        return items.value
      } catch (err) {
        if (currentVersion !== version || signal.aborted) return items.value
        error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
        throw err
      } finally {
        if (currentVersion === version) {
          loading.value = false
          latestPromise = undefined
          streamDuringRead = undefined
          scheduleReconcile()
        }
      }
    })()
    return latestPromise
  }

  async function loadOlder() {
    if (!olderCursor.value || loadingOlder.value || loading.value || reconciling) return items.value
    const currentVersion = version
    const signal = controller.signal
    loadingOlder.value = true
    error.value = null
    try {
      const response = await requestPage(signal, olderCursor.value)
      if (currentVersion !== version) return items.value
      items.value = mergeSortedLogItemsAsc(items.value, normalizeLogListResponseItems(response))
      olderCursor.value = response.page?.older_cursor ?? null
      hasOlder.value = Boolean(response.page?.has_older)
      return items.value
    } catch (err) {
      if (currentVersion !== version || signal.aborted) return items.value
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      if (currentVersion === version) {
        loadingOlder.value = false
        scheduleReconcile()
      }
    }
  }

  function append(log: LogSummary) {
    return appendBatch([log]) > 0
  }

  function appendBatch(logs: LogSummary[]) {
    if (logs.length === 0) return 0
    if (!active.value || document.visibilityState === 'hidden') {
      needsReload = true
      return 0
    }
    // WS delivery can replace queued frames. HTTP confirms continuity independently of received IDs.
    dirty = true
    scheduleReconcile()
    let matching = logs.filter(createLogFilterMatcher(filters.value))
    if (matching.length === 0) return 0
    if (streamDuringRead) {
      for (const log of matching) streamDuringRead.push(log)
    }
    const raw = toRaw(items.value)
    if (raw !== indexedItems || raw.length !== indexedLength) {
      identities = new Set(raw.map(getLogIdentityKey))
      indexedItems = raw
      indexedLength = raw.length
    }
    // Replay may arrive after HTTP hydration. Keep the loaded window contiguous until older paging
    // extends it; a replacement HTTP window must first collect frames without using the old boundary.
    const oldest = raw[0]
    if (initialized.value && hasOlder.value && !loading.value && oldest) {
      matching = matching.filter(log =>
        compareLogTimestamps(log, oldest) >= 0 || identities.has(getLogIdentityKey(log)),
      )
      if (matching.length === 0) return 0
    }
    let added = 0
    let ordered = true
    let last = raw.at(-1)
    for (const log of matching) {
      const key = getLogIdentityKey(log)
      if (identities.has(key)) ordered = false
      else { identities.add(key); added += 1 }
      if (last && compareLogItems(last, log) >= 0) ordered = false
      last = log
    }
    if (ordered && canAppendInPlace(raw, matching[0]!)) {
      for (let offset = 0; offset < matching.length; offset += 1024) {
        items.value.push(...matching.slice(offset, offset + 1024))
      }
      indexedLength = items.value.length
    } else {
      items.value = mergeSortedLogItemsAsc(raw, matching)
      indexedItems = toRaw(items.value)
      indexedLength = items.value.length
    }
    pendingNewCount.value = atBottom.value ? 0 : pendingNewCount.value + added
    return added
  }

  function scheduleReconcile() {
    if (document.visibilityState === 'hidden' || !active.value || !initialized.value || loading.value || loadingOlder.value || reconciling || !dirty || reconcileTimer !== undefined) return
    reconcileTimer = setTimeout(() => {
      reconcileTimer = undefined
      void reconcile().catch(() => undefined)
    }, reconcileIntervalMs)
  }

  async function reconcile() {
    if (!active.value || document.visibilityState === 'hidden' || !dirty || loading.value || loadingOlder.value || reconciling) return
    const currentVersion = version
    const signal = controller.signal
    const anchor = confirmedNewest
    dirty = false
    reconciling = true
    streamDuringRead = []
    try {
      let response = await requestPage(signal)
      if (currentVersion !== version) return
      let next = normalizeLogListResponseItems(response)
      const newest = next.at(-1)?.log_id
      let found = Boolean(anchor && next.some(log => log.log_id === anchor))
      // Only page back to the previous HTTP boundary; retained history need not be fetched again.
      while (!found && response.page?.has_older && response.page.older_cursor) {
        await yieldPage(signal)
        response = await requestPage(signal, response.page.older_cursor)
        if (currentVersion !== version) return
        const page = normalizeLogListResponseItems(response)
        found = page.some(log => log.log_id === anchor)
        next.push(...page)
      }
      next = mergeSortedLogItemsAsc([], next)
      if (found) {
        const previousCount = items.value.length
        items.value = mergeSortedLogItemsAsc(items.value, next)
        if (!atBottom.value) pendingNewCount.value += Math.max(0, items.value.length - previousCount)
      } else {
        items.value = mergeWindowStream(next, streamDuringRead ?? [], Boolean(response.page?.has_older))
        olderCursor.value = response.page?.older_cursor ?? null
        hasOlder.value = Boolean(response.page?.has_older)
      }
      confirmedNewest = newest
      error.value = null
    } catch (err) {
      if (currentVersion !== version || signal.aborted) return
      dirty = true
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
    } finally {
      if (currentVersion === version) {
        reconciling = false
        streamDuringRead = undefined
        scheduleReconcile()
      }
    }
  }

  function onStreamConnected() {
    needsReload = true
    if (active.value && document.visibilityState !== 'hidden') void fetchWindow(Math.max(1, Math.ceil(items.value.length / pageLimit)), false).catch(() => undefined)
  }

  function setViewportActive(nextValue: boolean) {
    active.value = nextValue
    if (!nextValue) {
      needsReload = initialized.value || needsReload || loading.value
      resetRequests()
    }
  }

  function setViewportAtBottom(nextValue: boolean) {
    atBottom.value = nextValue
    if (nextValue) pendingNewCount.value = 0
  }

  function acknowledgePendingNew() { pendingNewCount.value = 0 }

  function onVisibilityChange() {
    if (!active.value) return
    if (document.visibilityState === 'hidden') {
      needsReload = true
      resetRequests()
    } else {
      void ensureLoaded().catch(() => undefined)
    }
  }
  document.addEventListener('visibilitychange', onVisibilityChange)
  onScopeDispose(() => {
    resetRequests()
    document.removeEventListener('visibilitychange', onVisibilityChange)
  })

  return {
    active, atBottom, error, filters, hasOlder, initialized, items, loading, loadingOlder, pendingNewCount,
    acknowledgePendingNew, append, appendBatch, applyFilters, ensureLoaded, loadOlder, onStreamConnected,
    setViewportActive, setViewportAtBottom,
  }
})

function mergeWindowStream(window: LogSummary[], stream: LogSummary[], hasOlder: boolean) {
  // Old replay rows outside the HTTP window remain accessible through its opaque older cursor.
  const oldest = window[0]
  if (!hasOlder || !oldest || stream.length === 0) return mergeSortedLogItemsAsc(window, stream)
  const windowIDs = new Set(window.map(getLogIdentityKey))
  return mergeSortedLogItemsAsc(window, stream.filter(log =>
    compareLogTimestamps(log, oldest) >= 0 || windowIDs.has(getLogIdentityKey(log)),
  ))
}

function yieldPage(signal: AbortSignal) {
  return new Promise<void>((resolve, reject) => {
    if (signal.aborted) { reject(signal.reason); return }
    const abort = () => { clearTimeout(timer); reject(signal.reason) }
    const timer = setTimeout(() => { signal.removeEventListener('abort', abort); resolve() }, 0)
    signal.addEventListener('abort', abort, { once: true })
  })
}
