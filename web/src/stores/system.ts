import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiDownload, apiRequest } from '@/lib/http'
import { formatDashboardEventSummary } from '@/lib/management-summary'
import type {
  EventsPayload,
  LivenessStatusResponse,
  RecoveryConfirmRequest,
  ReadinessStatusResponse,
  RuntimeBootstrapResource,
  TaskAcceptedResponse,
  SystemShutdownResponse,
  SystemStatusResponse,
} from '@/types/api'
import { t } from '@/i18n'

export const useSystemStore = defineStore('system', () => {
  const health = ref<LivenessStatusResponse | null>(null)
  const readiness = ref<ReadinessStatusResponse | null>(null)
  const system = ref<SystemStatusResponse | null>(null)
  const loading = ref(false)
  const shutdownPending = ref(false)
  const shutdownRequested = ref(false)
  const backupPending = ref(false)
  const diagnosticsPending = ref(false)
  const recoveryRecheckPending = ref(false)
  const recoveryConfirmPending = ref(false)
  const runtimeBootstrapPending = ref(false)
  const error = ref<string | null>(null)
  const recentEvents = ref<Array<{ timestamp: string; summary: string; payload: EventsPayload }>>([])

  const isHealthy = computed(() => health.value?.status === 'ok')

  async function requestReadinessStatus() {
    return await apiRequest<ReadinessStatusResponse>('/readyz', {
      auth: false,
      acceptStatuses: [503],
    })
  }

  async function refreshSnapshot(options: { includeHealth: boolean; interactive: boolean }) {
    if (options.interactive) {
      loading.value = true
      error.value = null
    }
    try {
      const requests = [
        requestReadinessStatus(),
        apiRequest<SystemStatusResponse>('/api/system/status'),
      ] as const

      if (options.includeHealth) {
        const [nextHealth, nextReadiness, nextSystem] = await Promise.all([
          apiRequest<LivenessStatusResponse>('/healthz', { auth: false }),
          ...requests,
        ])
        health.value = nextHealth
        readiness.value = nextSystem.health ?? nextReadiness
        system.value = nextSystem
        return
      }

      const [nextReadiness, nextSystem] = await Promise.all(requests)
      readiness.value = nextSystem.health ?? nextReadiness
      system.value = nextSystem
    } catch (err) {
      if (options.interactive) {
        error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      }
      throw err
    } finally {
      if (options.interactive) {
        loading.value = false
      }
    }
  }

  async function refreshAll() {
    await refreshSnapshot({ includeHealth: true, interactive: true })
  }

  async function refreshStatus() {
    await refreshSnapshot({ includeHealth: false, interactive: false })
  }

  function applyEvent(timestamp: string, payload: EventsPayload) {
    const summary = formatDashboardEventSummary(payload)
    if (!summary) {
      return
    }

    recentEvents.value = [{ timestamp, summary, payload }, ...recentEvents.value].slice(0, 12)
  }

  async function requestShutdown() {
    shutdownPending.value = true
    error.value = null
    try {
      const response = await apiRequest<SystemShutdownResponse>('/api/system/shutdown', {
        method: 'POST',
      })
      shutdownRequested.value = response.accepted
      if (response.accepted && system.value) {
        system.value = {
          ...system.value,
          status: 'shutting_down',
        }
      }
      return response
    } finally {
      shutdownPending.value = false
    }
  }

  async function createBackup() {
    backupPending.value = true
    error.value = null
    try {
      return await apiRequest<TaskAcceptedResponse>('/api/system/backup', {
        method: 'POST',
      })
    } finally {
      backupPending.value = false
    }
  }

  async function exportDiagnostics() {
    diagnosticsPending.value = true
    error.value = null
    try {
      const { blob, filename } = await apiDownload('/api/system/diagnostics/export')
      const objectURL = window.URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = objectURL
      anchor.download = filename ?? 'rayleabot-diagnostics.zip'
      anchor.style.display = 'none'
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
      window.URL.revokeObjectURL(objectURL)
    } finally {
      diagnosticsPending.value = false
    }
  }

  async function recheckRecovery() {
    recoveryRecheckPending.value = true
    error.value = null
    try {
      return await apiRequest<TaskAcceptedResponse>('/api/system/recovery/recheck', {
        method: 'POST',
      })
    } finally {
      recoveryRecheckPending.value = false
    }
  }

  async function confirmRecovery(request: RecoveryConfirmRequest) {
    recoveryConfirmPending.value = true
    error.value = null
    try {
      return await apiRequest<TaskAcceptedResponse>('/api/system/recovery/confirm', {
        method: 'POST',
        body: request,
      })
    } finally {
      recoveryConfirmPending.value = false
    }
  }

  async function bootstrapManagedRuntime(resources?: RuntimeBootstrapResource[]) {
    runtimeBootstrapPending.value = true
    error.value = null
    try {
      return await apiRequest<TaskAcceptedResponse>('/api/system/runtime/bootstrap', {
        method: 'POST',
        body: resources?.length ? { resources } : undefined,
      })
    } finally {
      runtimeBootstrapPending.value = false
    }
  }

  return {
    backupPending,
    bootstrapManagedRuntime,
    confirmRecovery,
    diagnosticsPending,
    error,
    health,
    isHealthy,
    loading,
    readiness,
    recoveryConfirmPending,
    recoveryRecheckPending,
    recentEvents,
    recheckRecovery,
    shutdownPending,
    shutdownRequested,
    system,
    runtimeBootstrapPending,
    applyEvent,
    createBackup,
    exportDiagnostics,
    refresh: refreshAll,
    refreshAll,
    refreshStatus,
    requestShutdown,
  }
})
