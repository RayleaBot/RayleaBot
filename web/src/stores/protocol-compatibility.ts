import { ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type { OneBot11ProtocolCompatibilityResponse } from '@/types/api'

export const useProtocolCompatibilityStore = defineStore('protocol-compatibility', () => {
  const matrix = ref<OneBot11ProtocolCompatibilityResponse | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function refresh() {
    loading.value = true
    error.value = null
    try {
      const nextMatrix = await apiRequest<OneBot11ProtocolCompatibilityResponse>('/api/protocols/onebot11/compatibility')
      matrix.value = nextMatrix
      return { matrix: nextMatrix }
    } catch (err) {
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      loading.value = false
    }
  }

  return {
    error,
    loading,
    matrix,
    refresh,
  }
})
