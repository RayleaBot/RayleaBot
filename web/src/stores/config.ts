import { ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type { ConfigApplyEffects, ConfigDocument, ConfigSnapshotResponse, ConfigUpdateResponse } from '@/types/api'

export const useConfigStore = defineStore('config', () => {
  const document = ref<ConfigDocument | null>(null)
  const applyEffects = ref<ConfigApplyEffects | null>(null)
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
      applyEffects.value = null
      redactedFields.value = response.redacted_fields ?? []
      restartRequired.value = null
    } catch (err) {
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
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
      applyEffects.value = response.apply_effects
      redactedFields.value = response.redacted_fields ?? []
      restartRequired.value = response.restart_required
      return response
    } catch (err) {
      error.value = getDisplayErrorMessage(err)
      throw err
    } finally {
      saving.value = false
    }
  }

  return {
    applyEffects,
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
