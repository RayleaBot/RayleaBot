import { computed, onScopeDispose, ref, shallowRef, watch, type Ref } from 'vue'
import { useDocumentVisibility, useIntervalFn } from '@vueuse/core'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import { useConfigStore } from '@/stores/config'
import { useMessageStatsLiveStore } from '@/stores/message-stats-live'
import { useSocketStore } from '@/stores/sockets'
import type { MessageStatsResponse } from '@/types/api'
import { localDate, localMidnight, periodWindow, shiftDate, type MessagePeriod } from '@/views/dashboard/message-stats'

// The server announces every change over /ws/events, at most every 2 seconds; one minute is the fallback when the
// stream is down or a notice was replaced on a slow connection.
const FALLBACK_REFRESH_MS = 60_000
// Rereads triggered by notices start at least this far apart; a notice that arrives mid-request queues one more.
const LIVE_MIN_GAP_MS = 1_000
// A new hour or day opens a new bucket even when nothing is sent; read it just after the boundary.
const BOUNDARY_DELAY_MS = 1_500

export function useMessageStats(period: Ref<MessagePeriod>) {
  const stats = shallowRef<MessageStatsResponse | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const updatedAt = ref<number | null>(null)
  // Counts that grew since the previous reading of the same period; the card marks them with a pulse.
  const pulse = ref(0)
  const configStore = useConfigStore()
  const liveStore = useMessageStatsLiveStore()
  const socketStore = useSocketStore()
  const visibility = useDocumentVisibility()
  let sequence = 0
  let controller: AbortController | null = null
  let inFlight = false
  let queued = false
  let lastStart = 0
  let liveTimer: ReturnType<typeof setTimeout> | undefined
  let boundaryTimer: ReturnType<typeof setTimeout> | undefined

  // A window that ends after the latest reading is still being counted, so it can change.
  const open = computed(() => Boolean(stats.value && Date.parse(stats.value.end_at) > Date.parse(stats.value.as_of)))
  const live = computed(() => open.value && socketStore.snapshots.events.status === 'authenticated')

  async function load() {
    const current = ++sequence
    controller?.abort()
    controller = new AbortController()
    const window = periodWindow(period.value, new Date(), configStore.effectiveTimezone)
    const query = new URLSearchParams({ start_at: window.startAt, end_at: window.endAt, granularity: window.granularity })
    loading.value = true
    inFlight = true
    lastStart = Date.now()
    try {
      const response = await apiRequest<MessageStatsResponse>(`/api/system/message-stats?${query}`, { signal: controller.signal })
      if (current !== sequence) return
      const previous = stats.value
      if (previous && previous.start_at === response.start_at && total(response) > total(previous)) pulse.value++
      stats.value = response
      error.value = null
      updatedAt.value = Date.now()
      scheduleBoundary(response)
    } catch (err) {
      if (current !== sequence) return
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
    } finally {
      if (current === sequence) {
        loading.value = false
        inFlight = false
        if (queued) {
          queued = false
          refreshSoon()
        }
      }
    }
  }

  function refreshSoon() {
    if (inFlight) {
      queued = true
      return
    }
    clearTimeout(liveTimer)
    liveTimer = setTimeout(() => void load(), Math.max(0, lastStart + LIVE_MIN_GAP_MS - Date.now()))
  }

  function scheduleBoundary(response: MessageStatsResponse) {
    clearTimeout(boundaryTimer)
    if (Date.parse(response.end_at) <= Date.parse(response.as_of)) return
    const now = Date.now()
    const next = response.granularity === 'hour'
      ? Math.floor(now / 3_600_000) * 3_600_000 + 3_600_000
      : Date.parse(localMidnight(shiftDate(localDate(new Date(now), response.timezone), 1), response.timezone))
    if (next >= Date.parse(response.end_at)) return
    boundaryTimer = setTimeout(refreshSoon, next - now + BOUNDARY_DELAY_MS)
  }

  // A new period or time zone replaces the old figures at once; keeping them would label old numbers with the new period.
  watch([period, () => configStore.effectiveTimezone], () => {
    stats.value = null
    queued = false
    clearTimeout(liveTimer)
    clearTimeout(boundaryTimer)
    void load()
  }, { deep: true, immediate: true })
  watch(() => liveStore.changedAt, () => {
    if (open.value && visibility.value === 'visible') refreshSoon()
  })
  useIntervalFn(() => {
    if (visibility.value === 'visible' && open.value && Date.now() - (updatedAt.value ?? 0) >= FALLBACK_REFRESH_MS) refreshSoon()
  }, FALLBACK_REFRESH_MS / 4)
  watch(visibility, (state) => {
    if (state === 'visible' && open.value && Date.now() - (updatedAt.value ?? 0) >= LIVE_MIN_GAP_MS) refreshSoon()
  })
  onScopeDispose(() => {
    controller?.abort()
    clearTimeout(liveTimer)
    clearTimeout(boundaryTimer)
  })

  return { stats, loading, error, open, live, updatedAt, pulse, reload: load }
}

function total(stats: MessageStatsResponse) {
  return stats.totals.received + stats.totals.sent
}
