import { onMounted, ref } from 'vue'

type DashboardRefreshInput = {
  adaptersStore: {
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
        await input.adaptersStore.refresh()
      } catch {
        // protocol store error state is optional on the dashboard
      }
      lastRefreshed.value = new Date().toISOString()
    } catch {
      // store error state drives the page
    }
  }

  onMounted(() => {
    void refreshState()
  })

  return {
    lastRefreshed,
    refreshState,
  }
}
