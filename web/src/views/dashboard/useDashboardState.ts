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
    recoveryConfirmPending,
    recoveryRecheckPending,
    runtimeBootstrapPending,
    system,
  } = storeToRefs(systemStore)
  const { adapters } = storeToRefs(adaptersStore)

  const issuesExpanded = ref(false)
  const eventsExpanded = ref(false)
  const selectedRecoveryReviewIds = ref<string[]>([])
  const recoveryConfirmNote = ref('')

  const derivedState = useDashboardDerivedState({
    diagnostics,
    health,
    readiness,
    selectedRecoveryReviewIds,
    system,
  })
  const refreshState = useDashboardRefresh({
    adaptersStore,
    recoveryConfirmNote,
    recoverySummary: derivedState.recoverySummary,
    selectedRecoveryReviewIds,
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
    recoveryConfirmNote,
    recoveryConfirmPending,
    recoveryRecheckPending,
    runtimeBootstrapPending,
    selectedRecoveryReviewIds,
    system,
    systemStore,
  }
}
