import { onMounted, ref } from 'vue'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type { UpdateStatusResponse } from '@/types/api'

// The formal update state: read once when the status page opens, re-checked on demand. Web never downloads
// or installs updates; the guidance names the Launcher and `raylea-server update apply`.
export function useUpdateStatus() {
  const status = ref<UpdateStatusResponse | null>(null)
  const checking = ref(false)
  const error = ref<string | null>(null)

  async function fetchStatus() {
    error.value = null
    try {
      status.value = await apiRequest<UpdateStatusResponse>('/api/update/status')
    } catch (requestError) {
      error.value = getDisplayErrorMessage(requestError)
    }
  }

  async function check() {
    checking.value = true
    error.value = null
    try {
      status.value = await apiRequest<UpdateStatusResponse>('/api/update/check', { method: 'POST' })
    } catch (requestError) {
      const message = getDisplayErrorMessage(requestError)
      await fetchStatus()
      error.value = message
    } finally {
      checking.value = false
    }
  }

  onMounted(() => {
    void fetchStatus()
  })

  return { check, checking, error, status }
}
