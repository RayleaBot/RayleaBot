import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiDownload, apiRequest } from '@/lib/http'
import type {
  EventsPayload,
  LivenessStatusResponse,
  ReadinessStatusResponse,
  RenderPreviewRequest,
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
  const runtimeBootstrapPending = ref(false)
  const previewPending = ref(false)
  const error = ref<string | null>(null)
  const recentEvents = ref<Array<{ timestamp: string; summary: string; payload: EventsPayload }>>([])

  const isHealthy = computed(() => health.value?.status === 'ok')

  async function refresh() {
    loading.value = true
    error.value = null
    try {
      const [nextHealth, nextReadiness, nextSystem] = await Promise.all([
        apiRequest<LivenessStatusResponse>('/healthz', { auth: false }),
        apiRequest<ReadinessStatusResponse>('/readyz', { auth: false }),
        apiRequest<SystemStatusResponse>('/api/system/status'),
      ])

      health.value = nextHealth
      readiness.value = nextReadiness
      system.value = nextSystem
    } catch (err) {
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      loading.value = false
    }
  }

  function applyEvent(timestamp: string, payload: EventsPayload) {
    let summary = '管理事件'
    if ('summary' in payload) {
      summary = payload.summary
    } else if ('plugin_id' in payload) {
      summary = `${payload.plugin_id} -> ${payload.runtime_state}`
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

  async function previewRender(request: RenderPreviewRequest) {
    previewPending.value = true
    error.value = null
    try {
      return await apiRequest<TaskAcceptedResponse>('/api/system/render/preview', {
        method: 'POST',
        body: request,
      })
    } finally {
      previewPending.value = false
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
    diagnosticsPending,
    error,
    health,
    isHealthy,
    loading,
    readiness,
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
    previewPending,
    previewRender,
    refresh,
    requestShutdown,
  }
})
