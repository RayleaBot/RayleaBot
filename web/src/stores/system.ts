import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { apiRequest } from '@/lib/http'
import type {
  EventsPayload,
  LivenessStatusResponse,
  ReadinessStatusResponse,
  SystemShutdownResponse,
  SystemStatusResponse,
} from '@/types/api'

export const useSystemStore = defineStore('system', () => {
  const health = ref<LivenessStatusResponse | null>(null)
  const readiness = ref<ReadinessStatusResponse | null>(null)
  const system = ref<SystemStatusResponse | null>(null)
  const loading = ref(false)
  const shutdownPending = ref(false)
  const shutdownRequested = ref(false)
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
      error.value = err instanceof Error ? err.message : 'system refresh failed'
      throw err
    } finally {
      loading.value = false
    }
  }

  function applyEvent(timestamp: string, payload: EventsPayload) {
    let summary = 'management event'
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

  return {
    error,
    health,
    isHealthy,
    loading,
    readiness,
    recentEvents,
    shutdownPending,
    shutdownRequested,
    system,
    applyEvent,
    refresh,
    requestShutdown,
  }
})
