import { onMounted, onUnmounted, ref, watch, type ComputedRef, type Ref } from 'vue'

import { AUTO_REFRESH_INTERVAL } from '@/pages/dashboard/constants'

type DashboardRefreshInput = {
  recoveryConfirmNote: Ref<string>
  recoverySummary: ComputedRef<any>
  selectedRecoveryReviewIds: Ref<string[]>
  systemStore: {
    refresh: () => Promise<void>
  }
}

export function useDashboardRefresh(input: DashboardRefreshInput) {
  const autoRefresh = ref(false)
  const lastRefreshed = ref<string | null>(null)
  const countdown = ref(AUTO_REFRESH_INTERVAL)
  let autoRefreshTimer: ReturnType<typeof setInterval> | null = null
  let countdownTimer: ReturnType<typeof setInterval> | null = null

  async function refreshState() {
    try {
      await input.systemStore.refresh()
      lastRefreshed.value = new Date().toISOString()
      countdown.value = AUTO_REFRESH_INTERVAL
    } catch {
      // store error state drives the page
    }
  }

  function startAutoRefresh() {
    stopAutoRefresh()
    autoRefresh.value = true
    countdown.value = AUTO_REFRESH_INTERVAL

    countdownTimer = setInterval(() => {
      countdown.value = Math.max(0, countdown.value - 1)
    }, 1000)

    autoRefreshTimer = setInterval(() => {
      void refreshState()
    }, AUTO_REFRESH_INTERVAL * 1000)
  }

  function stopAutoRefresh() {
    if (autoRefreshTimer !== null) {
      clearInterval(autoRefreshTimer)
      autoRefreshTimer = null
    }
    if (countdownTimer !== null) {
      clearInterval(countdownTimer)
      countdownTimer = null
    }
    autoRefresh.value = false
  }

  function toggleAutoRefresh(val: boolean) {
    if (val) {
      void refreshState()
      startAutoRefresh()
      return
    }
    stopAutoRefresh()
  }

  watch(input.recoverySummary, (nextSummary) => {
    const pendingIds = new Set(
      (nextSummary?.skipped_plugins ?? [])
        .filter((plugin: any) => plugin.review_status !== 'confirmed')
        .map((plugin: any) => plugin.review_id),
    )
    input.selectedRecoveryReviewIds.value = input.selectedRecoveryReviewIds.value.filter(reviewID => pendingIds.has(reviewID))
    if (input.selectedRecoveryReviewIds.value.length === 0) {
      input.recoveryConfirmNote.value = ''
    }
  })

  onMounted(() => {
    void refreshState()
  })

  onUnmounted(() => {
    stopAutoRefresh()
  })

  return {
    autoRefresh,
    countdown,
    lastRefreshed,
    refreshState,
    toggleAutoRefresh,
  }
}
