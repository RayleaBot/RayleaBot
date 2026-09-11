import { computed, nextTick, onActivated, onDeactivated, onMounted, onUnmounted, ref, watch, type Ref } from 'vue'
import { storeToRefs } from 'pinia'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'
import { useToastFeedback } from '@/adapter/feedback'
import { useHeavyContentGate } from '@/layouts/usePageTransitionStage'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { areLocationQueriesEqual, buildLogsLocation, readLogWorkspaceState } from '@/lib/management-links'
import { sameLogFilters } from '@/stores/log-state'
import { toLocalDateTimeInput, useLogHistoryStore } from '@/stores/log-history'
import { useLogsStore } from '@/stores/logs'
import { useUiShellStore } from '@/stores/ui-shell'
import type { LogSummary } from '@/types/api'
import { useLogDetailController } from './useLogDetailController'

export type LogWorkspaceScope = 'current_session' | 'history'

export interface LogViewport {
  getScrollMetrics?: () => { clientHeight: number; scrollHeight: number; scrollTop: number }
  scrollToBottom: () => void
}

// A route owns one fixed scope. Both sources share selection, routing, paging and viewport lifecycle.
export function useLogWorkspace(scope: LogWorkspaceScope, viewportRef: Ref<LogViewport | null>) {
  const history = scope === 'history'
  const routeName = history ? 'logs-history' : 'logs'
  const route = useRoute()
  const router = useRouter()
  const uiShellStore = useUiShellStore()
  const historyStore = history ? useLogHistoryStore() : null
  const liveStore = history ? null : useLogsStore()
  const store = historyStore ?? liveStore!
  const { filters, hasOlder, initialized, items, loading, loadingOlder } = storeToRefs(store)
  const detail = useLogDetailController()
  const { readyToRenderHeavyContent, waitUntilReady } = useHeavyContentGate()
  const restoringLatest = ref(false)
  const historyFollowBottom = ref(false)
  const operationError = ref<string | null>(null)
  const error = computed(() => operationError.value ?? store.error)
  const atBottom = computed(() => liveStore?.atBottom ?? false)
  const followBottom = computed(() => liveStore?.atBottom ?? historyFollowBottom.value)
  const pendingNewCount = computed(() => liveStore?.pendingNewCount ?? 0)
  const showJumpToLatest = computed(() => !history && readyToRenderHeavyContent.value
    && initialized.value && !restoringLatest.value && !atBottom.value)
  let routeSyncing = false
  let routeVersion = 0
  let viewportVersion = 0
  let activation: Promise<void> | null = null

  useToastFeedback(computed(() => error.value ? {
    key: `${routeName}-error:${error.value}`, level: 'error' as const, message: error.value,
  } : null))

  function routeState() {
    return readLogWorkspaceState(route.query, { history })
  }

  async function run(operation: () => Promise<unknown>) {
    operationError.value = null
    try {
      await operation()
      return true
    } catch (failure) {
      operationError.value = getDisplayErrorMessage(failure, 'errors.common.loadFailed')
      return false
    }
  }

  async function replaceRouteState(logId: string | null = detail.selectedLogId.value) {
    if (route.name !== routeName) return
    const range = historyStore?.currentUtcRange()
    const target = buildLogsLocation({ history, filters: filters.value, logId,
      startAt: range?.startAt ?? '', endAt: range?.endAt ?? '' })
    if (areLocationQueriesEqual(route.query, target.query ?? {})) return
    routeSyncing = true
    try {
      await router.replace(target)
    } finally {
      routeSyncing = false
    }
  }

  function cancelViewportSync() {
    viewportVersion += 1
    historyFollowBottom.value = false
    restoringLatest.value = false
  }

  function canFollowLatest() {
    return route.name === routeName && (!history || (items.value.length > 0 && !routeState().logId))
  }

  async function scrollToLatest() {
    if (!canFollowLatest()) {
      cancelViewportSync()
      return
    }
    const version = ++viewportVersion
    restoringLatest.value = true
    historyFollowBottom.value = history
    liveStore?.setViewportAtBottom(true)
    liveStore?.acknowledgePendingNew()
    const current = () => version === viewportVersion && canFollowLatest()
    let stablePasses = 0
    try {
      if (!await waitUntilReady()) return
      // Virtual rows can change height after mounting; require two measured stable frames.
      for (let attempt = 0; attempt < 6; attempt += 1) {
        await nextTick()
        if (!current()) return
        if (typeof window.requestAnimationFrame === 'function') {
          await new Promise<void>(resolve => window.requestAnimationFrame(() => resolve()))
        }
        if (!current()) return
        viewportRef.value?.scrollToBottom()
        await nextTick()
        if (!current()) return
        const metrics = viewportRef.value?.getScrollMetrics?.()
        if (!metrics || metrics.clientHeight < 1) {
          stablePasses = 0
          continue
        }
        const distance = Math.max(0, metrics.scrollHeight - metrics.clientHeight - metrics.scrollTop)
        stablePasses = distance <= 1 ? stablePasses + 1 : 0
        if (stablePasses >= 2) return
      }
    } finally {
      if (version === viewportVersion) {
        restoringLatest.value = false
        historyFollowBottom.value = history && canFollowLatest()
      }
    }
  }

  function toLocalInput(value: string) {
    const date = new Date(value)
    return value && !Number.isNaN(date.getTime()) ? toLocalDateTimeInput(date) : ''
  }

  async function loadRouteFilters(version: number) {
    const state = routeState()
    const filtersChanged = !sameLogFilters(filters.value, state.filters)
    if (filtersChanged) filters.value = { ...state.filters }
    if (historyStore) {
      if (state.startAt && state.endAt) {
        const range = { startLocal: toLocalInput(state.startAt), endLocal: toLocalInput(state.endAt) }
        const changed = historyStore.timeRangeInput.startLocal !== range.startLocal
          || historyStore.timeRangeInput.endLocal !== range.endLocal
        if (changed) historyStore.timeRangeInput = range
        if (filtersChanged || changed || !initialized.value) await historyStore.applyFilters()
      } else if (filtersChanged || !initialized.value) {
        historyStore.resetTimeRangeToDefault()
        await historyStore.refreshAnchor()
        if (version === routeVersion) await replaceRouteState(state.logId)
      }
    } else if (filtersChanged) {
      await store.applyFilters()
    } else {
      await liveStore!.ensureLoaded()
    }
    return state.logId
  }

  async function syncFromRoute() {
    if (route.name !== routeName) return
    const version = ++routeVersion
    const logId = await loadRouteFilters(version)
    if (version !== routeVersion || route.name !== routeName) return
    if (!logId) {
      detail.closeDetail()
      return
    }
    if (history) cancelViewportSync()
    const summary = items.value.find(item => item.log_id === logId)
    if (summary && detail.selectedLogId.value !== logId) {
      // The drawer owns detail errors; list loading must remain usable.
      await detail.openDetail(summary).catch(() => undefined)
    }
  }

  async function activatePage() {
    if (activation) return activation
    activation = run(async () => {
      liveStore?.setViewportActive(true)
      liveStore?.setViewportAtBottom(true)
      await syncFromRoute()
      await scrollToLatest()
    }).then(() => undefined)
    try { await activation } finally { activation = null }
  }

  async function applyFilters() {
    routeVersion += 1
    cancelViewportSync()
    liveStore?.setViewportAtBottom(true)
    return run(async () => {
      await store.applyFilters()
      detail.closeDetail()
      await replaceRouteState(null)
      await scrollToLatest()
    })
  }

  async function useRecentDays(days: number) {
    if (!historyStore) return false
    routeVersion += 1
    cancelViewportSync()
    if (days === 1) historyStore.resetTimeRangeToDefault()
    else historyStore.setTimeRange(days)
    return run(async () => {
      await historyStore.refreshAnchor()
      await replaceRouteState()
      await scrollToLatest()
    })
  }

  async function loadOlder() {
    if (hasOlder.value && !loadingOlder.value) await run(() => store.loadOlder())
  }

  async function openLogDetail(summary: LogSummary) {
    if (history) cancelViewportSync()
    const request = detail.openDetail(summary).catch(() => undefined)
    await run(() => replaceRouteState(summary.log_id))
    await request
  }

  async function closeLogDetail() {
    detail.closeDetail()
    await run(() => replaceRouteState(null))
  }

  function onViewportBottomChange(value: boolean) {
    if (!restoringLatest.value || value) liveStore?.setViewportAtBottom(value)
  }

  function deactivate(syncTab: boolean) {
    routeVersion += 1
    cancelViewportSync()
    if (!liveStore) return
    liveStore.setViewportActive(false)
    liveStore.setViewportAtBottom(true)
    liveStore.acknowledgePendingNew()
    detail.closeDetail()
    if (syncTab) {
      const target = router.resolve(buildLogsLocation({ filters: filters.value, logId: null }))
      const tab = uiShellStore.tabs.find(item => item.path === target.path)
      if (tab && tab.fullPath !== target.fullPath) uiShellStore.upsertTab({ ...tab, fullPath: target.fullPath })
    }
  }

  watch(() => route.query, () => {
    if (!routeSyncing && route.name === routeName) void run(syncFromRoute)
  })
  onMounted(() => { void activatePage() })
  onActivated(() => { void activatePage() })
  if (!history) onBeforeRouteLeave(() => { deactivate(true) })
  onDeactivated(() => { deactivate(true) })
  onUnmounted(() => { deactivate(false) })

  return { historyStore, filters, initialized, items, loading, error, detail,
    readyToRenderHeavyContent, atBottom, followBottom, pendingNewCount, showJumpToLatest,
    activatePage, applyFilters, useRecentDays, loadOlder, scrollToLatest,
    openLogDetail, closeLogDetail, onViewportBottomChange }
}
