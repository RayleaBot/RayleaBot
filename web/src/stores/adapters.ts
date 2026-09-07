import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type { AdapterDescriptor, AdaptersResponse } from '@/types/api'

export const useAdaptersStore = defineStore('adapters', () => {
  const adapters = ref<AdapterDescriptor[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  // An adapter counts as added once it has been configured, whether or not it
  // is currently switched on. The rest are what the operator can add.
  const added = computed(() => adapters.value.filter((adapter) => adapter.configured))
  const available = computed(() => adapters.value.filter((adapter) => !adapter.configured))

  async function refresh() {
    loading.value = true
    error.value = null
    try {
      const response = await apiRequest<AdaptersResponse>('/api/adapters')
      adapters.value = response.adapters ?? []
      return response
    } catch (err) {
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      loading.value = false
    }
  }

  return { adapters, added, available, error, loading, refresh }
})
