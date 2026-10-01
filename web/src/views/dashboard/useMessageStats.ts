import { onScopeDispose, ref, shallowRef, watch, type Ref } from 'vue'
import { useDocumentVisibility, useIntervalFn } from '@vueuse/core'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import { useConfigStore } from '@/stores/config'
import type { MessageStatsResponse } from '@/types/api'
import { periodWindow, type MessagePeriod } from '@/views/dashboard/message-stats'

// The server flushes its counters every minute, so the open period is re-read once a minute while the page is shown.
const REFRESH_MS = 60_000

export function useMessageStats(period: Ref<MessagePeriod>) {
  const stats = shallowRef<MessageStatsResponse | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const configStore = useConfigStore()
  const visibility = useDocumentVisibility()
  let sequence = 0
  let controller: AbortController | null = null
  let loadedAt = 0

  async function load() {
    const current = ++sequence
    controller?.abort()
    controller = new AbortController()
    const window = periodWindow(period.value, new Date(), configStore.effectiveTimezone)
    const query = new URLSearchParams({ start_at: window.startAt, end_at: window.endAt, granularity: window.granularity })
    loading.value = true
    try {
      const response = await apiRequest<MessageStatsResponse>(`/api/system/message-stats?${query}`, { signal: controller.signal })
      if (current !== sequence) return
      stats.value = response
      error.value = null
      loadedAt = Date.now()
    } catch (err) {
      if (current !== sequence) return
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
    } finally {
      if (current === sequence) loading.value = false
    }
  }

  // A new period or time zone replaces the old figures at once; keeping them would label old numbers with the new period.
  watch([period, () => configStore.effectiveTimezone], () => {
    stats.value = null
    void load()
  }, { deep: true, immediate: true })
  useIntervalFn(() => {
    if (visibility.value === 'visible') void load()
  }, REFRESH_MS)
  watch(visibility, (state) => {
    if (state === 'visible' && Date.now() - loadedAt >= REFRESH_MS) void load()
  })
  onScopeDispose(() => controller?.abort())

  return { stats, loading, error, reload: load }
}
