import { ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type { AdapterDescriptor, AdapterProtocolDescriptor, AdaptersResponse } from '@/types/api'

export const useAdaptersStore = defineStore('adapters', () => {
  // Adapters are instances the operator added; protocols are what an instance
  // can be added for. Several instances may share one protocol, so the two
  // lists are independent rather than complements of each other.
  const adapters = ref<AdapterDescriptor[]>([])
  const availableProtocols = ref<AdapterProtocolDescriptor[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function refresh() {
    loading.value = true
    error.value = null
    try {
      const response = await apiRequest<AdaptersResponse>('/api/adapters')
      adapters.value = response.adapters ?? []
      availableProtocols.value = response.available_protocols ?? []
      return response
    } catch (err) {
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      loading.value = false
    }
  }

  // applySnapshot takes the live listing pushed over the socket. The set of
  // protocols one can add does not change at runtime, so only the instances
  // are replaced.
  function applySnapshot(next: AdapterDescriptor[]) {
    adapters.value = next ?? []
  }

  return { adapters, availableProtocols, error, loading, refresh, applySnapshot }
})
