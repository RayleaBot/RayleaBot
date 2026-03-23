import { ref } from 'vue'
import { defineStore } from 'pinia'

import { apiRequest } from '@/lib/http'
import type { ConfigDocument, ConfigSnapshotResponse, ConfigUpdateResponse } from '@/types/api'

export const useConfigStore = defineStore('config', () => {
  const document = ref<ConfigDocument | null>(null)
  const redactedFields = ref<string[]>([])
  const restartRequired = ref<boolean | null>(null)
  const loading = ref(false)
  const saving = ref(false)
  const error = ref<string | null>(null)

  async function fetchConfig() {
    loading.value = true
    error.value = null
    try {
      const response = await apiRequest<ConfigSnapshotResponse>('/api/config')
      document.value = response.config
      redactedFields.value = response.redacted_fields ?? []
      restartRequired.value = null
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'config load failed'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function saveConfig(nextDocument: ConfigDocument) {
    saving.value = true
    error.value = null
    try {
      const response = await apiRequest<ConfigUpdateResponse>('/api/config', {
        method: 'PUT',
        body: nextDocument,
      })
      document.value = response.config
      redactedFields.value = response.redacted_fields ?? []
      restartRequired.value = response.restart_required
      return response
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'config save failed'
      throw err
    } finally {
      saving.value = false
    }
  }

  return {
    document,
    error,
    loading,
    redactedFields,
    restartRequired,
    saving,
    fetchConfig,
    saveConfig,
  }
})
