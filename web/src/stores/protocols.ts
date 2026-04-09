import { ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type {
  OneBot11ProtocolCompatibilityResponse,
  OneBot11ProtocolSnapshotResponse,
} from '@/types/api'

export const useProtocolsStore = defineStore('protocols', () => {
  const snapshot = ref<OneBot11ProtocolSnapshotResponse | null>(null)
  const compatibility = ref<OneBot11ProtocolCompatibilityResponse | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function refresh() {
    loading.value = true
    error.value = null
    try {
      const [nextSnapshot, nextCompatibility] = await Promise.all([
        apiRequest<OneBot11ProtocolSnapshotResponse>('/api/protocols/onebot11'),
        apiRequest<OneBot11ProtocolCompatibilityResponse>('/api/protocols/onebot11/compatibility'),
      ])
      snapshot.value = nextSnapshot
      compatibility.value = nextCompatibility
      return { snapshot: nextSnapshot, compatibility: nextCompatibility }
    } catch (err) {
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      loading.value = false
    }
  }

  function applySnapshot(nextSnapshot: OneBot11ProtocolSnapshotResponse) {
    snapshot.value = nextSnapshot
  }

  function applyCompatibility(nextCompatibility: OneBot11ProtocolCompatibilityResponse) {
    compatibility.value = nextCompatibility
  }

  return {
    compatibility,
    error,
    loading,
    snapshot,
    applyCompatibility,
    applySnapshot,
    refresh,
  }
})
