import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'

import { useProtocolsStore } from '@/stores/protocols'
import { useSocketStore } from '@/stores/sockets'
import { useTasksStore } from '@/stores/tasks'
import { AUTO_REFRESH_INTERVAL } from '@/views/dashboard/constants'
import { useSystemStore } from '@/stores/system'
import { useDashboardDerivedState } from '@/views/dashboard/useDashboardDerivedState'
import { useDashboardRefresh } from '@/views/dashboard/useDashboardRefresh'

export function useDashboardState() {
  const router = useRouter()
  const protocolsStore = useProtocolsStore()
  const socketStore = useSocketStore()
  const systemStore = useSystemStore()
  const tasksStore = useTasksStore()
  const {
    backupPending,
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
  const { snapshot: protocolSnapshot } = storeToRefs(protocolsStore)

  const autoRefresh = ref(false)
  const lastRefreshed = ref<string | null>(null)
  const countdown = ref(AUTO_REFRESH_INTERVAL)
  const issuesExpanded = ref(false)
  const eventsExpanded = ref(false)
  const selectedRecoveryReviewIds = ref<string[]>([])
  const recoveryConfirmNote = ref('')

  const derivedState = useDashboardDerivedState({
    health,
    readiness,
    selectedRecoveryReviewIds,
    system,
  })
  const refreshState = useDashboardRefresh({
    eventsSocketStatus: computed(() => socketStore.snapshots.events.status),
    protocolsStore,
    recoveryConfirmNote,
    recoverySummary: derivedState.recoverySummary,
    selectedRecoveryReviewIds,
    systemStore,
  })

  return {
    ...derivedState,
    ...refreshState,
    backupPending,
    diagnosticsPending,
    error,
    health,
    eventsExpanded,
    issuesExpanded,
    loading,
    protocolSnapshot,
    protocolsStore,
    readiness,
    recentEvents,
    recoveryConfirmNote,
    recoveryConfirmPending,
    recoveryRecheckPending,
    router,
    runtimeBootstrapPending,
    selectedRecoveryReviewIds,
    system,
    systemStore,
    tasksStore,
  }
}
