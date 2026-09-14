import { ref } from 'vue'
import { storeToRefs } from 'pinia'

import { useAdaptersStore } from '@/stores/adapters'
import { useSystemStore } from '@/stores/system'
import { useDashboardDerivedState } from '@/views/dashboard/useDashboardDerivedState'
import { useDashboardRefresh } from '@/views/dashboard/useDashboardRefresh'

export function useDashboardState() {
  const adaptersStore = useAdaptersStore()
  const systemStore = useSystemStore()
  const {
    backupPending,
    diagnostics,
    diagnosticsPending,
    error,
    health,
    loading,
    readiness,
    recentEvents,
    runtimeBootstrapPending,
    system,
  } = storeToRefs(systemStore)
  const { adapters } = storeToRefs(adaptersStore)

  const issuesExpanded = ref(false)
  const eventsExpanded = ref(false)

  const derivedState = useDashboardDerivedState({
    diagnostics,
    health,
    readiness,
    system,
  })
  const refreshState = useDashboardRefresh({
    adaptersStore,
    systemStore,
  })

  return {
    ...derivedState,
    ...refreshState,
    backupPending,
    diagnostics,
    diagnosticsPending,
    error,
    health,
    eventsExpanded,
    issuesExpanded,
    loading,
    adapters,
    adaptersStore,
    readiness,
    recentEvents,
    runtimeBootstrapPending,
    system,
    systemStore,
  }
}
