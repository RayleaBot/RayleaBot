import { reactive, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'

import { useProtocolsStore } from '@/stores/protocols'
import { useTasksStore } from '@/stores/tasks'
import { AUTO_REFRESH_INTERVAL } from '@/views/dashboard/constants'
import { useSystemStore } from '@/stores/system'
import { useDashboardDerivedState } from '@/views/dashboard/useDashboardDerivedState'
import { useDashboardRefresh } from '@/views/dashboard/useDashboardRefresh'

export function useDashboardState() {
  const router = useRouter()
  const protocolsStore = useProtocolsStore()
  const systemStore = useSystemStore()
  const tasksStore = useTasksStore()
  const {
    backupPending,
    diagnosticsPending,
    error,
    health,
    loading,
    previewPending,
    readiness,
    recentEvents,
    recoveryConfirmPending,
    recoveryRecheckPending,
    runtimeBootstrapPending,
    system,
  } = storeToRefs(systemStore)
  const { snapshot: protocolSnapshot } = storeToRefs(protocolsStore)

  const previewVisible = ref(false)
  const previewForm = reactive({
    template: 'help.menu',
    theme: 'default',
    output: 'png' as 'png' | 'jpeg',
    dataText: JSON.stringify(
      {
        title: '帮助菜单',
        subtitle: '系统页渲染调试入口',
        items: [
          {
            name: 'weather',
            description: '查询天气',
            usage: '/weather <城市>',
          },
        ],
      },
      null,
      2,
    ),
  })

  const autoRefresh = ref(false)
  const lastRefreshed = ref<string | null>(null)
  const countdown = ref(AUTO_REFRESH_INTERVAL)
  const issuesExpanded = ref(false)
  const selectedRecoveryReviewIds = ref<string[]>([])
  const recoveryConfirmNote = ref('')

  const derivedState = useDashboardDerivedState({
    health,
    readiness,
    selectedRecoveryReviewIds,
    system,
  })
  const refreshState = useDashboardRefresh({
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
    issuesExpanded,
    loading,
    previewForm,
    previewPending,
    previewVisible,
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
