import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

export type ConnectionIssueSource = 'browser' | 'http' | 'websocket'

export const useAppAvailabilityStore = defineStore('app-availability', () => {
  const connectionIssueSource = ref<ConnectionIssueSource | null>(null)
  const lastConnectionIssueAt = ref<string | null>(null)

  const isConnectionInterrupted = computed(() => connectionIssueSource.value !== null)

  function markConnectionInterrupted(source: ConnectionIssueSource) {
    connectionIssueSource.value = source
    lastConnectionIssueAt.value = new Date().toISOString()
  }

  function markConnected() {
    connectionIssueSource.value = null
    lastConnectionIssueAt.value = null
  }

  return {
    connectionIssueSource,
    isConnectionInterrupted,
    lastConnectionIssueAt,
    markConnected,
    markConnectionInterrupted,
  }
})
