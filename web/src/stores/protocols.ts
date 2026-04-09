import { ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type { OneBot11ProtocolSnapshotResponse } from '@/types/api'

export const useProtocolsStore = defineStore('protocols', () => {
  const snapshot = ref<OneBot11ProtocolSnapshotResponse | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function refresh() {
    loading.value = true
    error.value = null
    try {
      const nextSnapshot = await apiRequest<OneBot11ProtocolSnapshotResponse>('/api/protocols/onebot11')
      snapshot.value = nextSnapshot
      return { snapshot: nextSnapshot }
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

  return {
    error,
    loading,
    snapshot,
    applySnapshot,
    refresh,
  }
})
