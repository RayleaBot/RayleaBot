import { onMounted, ref, watch, type ComputedRef, type Ref } from 'vue'

type DashboardRefreshInput = {
  recoveryConfirmNote: Ref<string>
  recoverySummary: ComputedRef<any>
  selectedRecoveryReviewIds: Ref<string[]>
  protocolsStore: {
    refresh: () => Promise<unknown>
  }
  systemStore: {
    refreshAll: () => Promise<void>
  }
}

export function useDashboardRefresh(input: DashboardRefreshInput) {
  const lastRefreshed = ref<string | null>(null)

  async function refreshState() {
    try {
      await input.systemStore.refreshAll()
      try {
        await input.protocolsStore.refresh()
      } catch {
        // protocol store error state is optional on the dashboard
      }
      lastRefreshed.value = new Date().toISOString()
    } catch {
      // store error state drives the page
    }
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

  return {
    lastRefreshed,
    refreshState,
  }
}
