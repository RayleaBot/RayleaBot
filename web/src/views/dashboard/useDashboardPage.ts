import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'

import { notifyError, notifySuccess, useToastFeedback } from '@/adapter/feedback'
import { t } from '@/i18n'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { useAdaptersStore } from '@/stores/adapters'
import { useSystemStore } from '@/stores/system'
import type { RuntimeBootstrapResource } from '@/types/api'
import {
  buildDiagnosticsIssueCards,
  buildDiagnosticsSubsystemItems,
  buildReadinessCheckItems,
  dedupeIssues,
  describeProtocolIssue,
  describeReadinessAlert,
  listUnexplainedReasonCodes,
} from './dashboard-status'

export function useDashboardPage() {
  const adaptersStore = useAdaptersStore()
  const systemStore = useSystemStore()
  const {
    backupPending,
    diagnostics,
    diagnosticsPending,
    error,
    loading,
    readiness,
    recentEvents,
    runtimeBootstrapPending,
    system,
  } = storeToRefs(systemStore)
  const { adapters } = storeToRefs(adaptersStore)

  const readinessIssues = computed(() => dedupeIssues(readiness.value?.issues))
  const checkItems = computed(() => buildReadinessCheckItems(readiness.value?.checks))
  const visibleReasonCodes = computed(() => listUnexplainedReasonCodes(readiness.value?.reason_codes, readinessIssues.value))
  const diagnosticsSubsystemItems = computed(() => buildDiagnosticsSubsystemItems(diagnostics.value))
  const diagnosticsIssueCards = computed(() => buildDiagnosticsIssueCards(diagnostics.value?.issues))

  // Uptime keeps counting between snapshots without refreshing dashboard data.
  const uptimeClock = ref(Date.now())
  const uptimeSnapshotAt = ref(Date.now())
  let uptimeTimer: number | null = null
  const liveUptimeSeconds = computed(() => {
    const baseUptime = system.value?.uptime_seconds
    if (baseUptime === undefined) return undefined
    return baseUptime + Math.max(0, Math.floor((uptimeClock.value - uptimeSnapshotAt.value) / 1000))
  })
  watch(() => system.value?.uptime_seconds, () => {
    uptimeClock.value = Date.now()
    uptimeSnapshotAt.value = uptimeClock.value
  }, { immediate: true })

  // The attention summary and the connection rows keep showing these problems; toasts only announce changes
  // that happen while the page is open.
  useToastFeedback(computed(() => {
    const alert = describeReadinessAlert(readiness.value, readinessIssues.value)
    if (!alert) return null
    return {
      key: `dashboard-readiness:${alert.level}:${alert.title}:${alert.detail}`,
      level: alert.level,
      message: alert.detail ? `${alert.title}：${alert.detail}` : alert.title,
    }
  }), { skipFirst: true })
  useToastFeedback(computed(() => error.value && system.value
    ? { key: `dashboard-error:${error.value}`, level: 'error' as const, message: error.value }
    : null))
  useToastFeedback(computed(() => {
    const issue = describeProtocolIssue(adapters.value)
    return issue
      ? { key: `dashboard-protocol:${issue.code}:${issue.summary}`, level: issue.level, message: issue.summary }
      : null
  }), { skipFirst: true })

  async function refreshState() {
    try {
      await systemStore.refreshAll()
      // The protocol snapshot only adds reminders; its failure must not hide the system state.
      await adaptersStore.refresh().catch(() => undefined)
    } catch {
      // The system store error drives the page.
    }
  }

  async function runAction(action: () => Promise<unknown>, acceptedKey: string) {
    try {
      await action()
      notifySuccess(t(acceptedKey))
    } catch (cause) {
      notifyError(getDisplayErrorMessage(cause))
    }
  }

  function createBackup() {
    return runAction(() => systemStore.createBackup(), 'dashboard.backupAccepted')
  }

  function exportDiagnostics() {
    return runAction(() => systemStore.exportDiagnostics(), 'dashboard.diagnosticsAccepted')
  }

  async function bootstrapRuntimeResources(resources: RuntimeBootstrapResource[]) {
    if (resources.length === 0) return
    await runAction(() => systemStore.bootstrapManagedRuntime(resources), 'dashboard.runtimeBootstrapAccepted')
  }

  onMounted(() => {
    void refreshState()
    uptimeTimer = window.setInterval(() => { uptimeClock.value = Date.now() }, 1000)
  })

  onBeforeUnmount(() => {
    if (uptimeTimer !== null) window.clearInterval(uptimeTimer)
    uptimeTimer = null
  })

  return {
    backupPending,
    bootstrapRuntimeResources,
    checkItems,
    createBackup,
    diagnosticsIssueCards,
    diagnosticsPending,
    diagnosticsSubsystemItems,
    error,
    exportDiagnostics,
    liveUptimeSeconds,
    loading,
    readinessIssues,
    recentEvents,
    refreshState,
    runtimeBootstrapPending,
    system,
    visibleReasonCodes,
  }
}
