import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type {
  BilibiliSourceRestartResponse,
  BilibiliSourceStatusResponse,
  ThirdPartyMonitorItem,
  ThirdPartyMonitorsResponse,
  ThirdPartyPlatform,
} from '@/types/api'

export const useThirdPartyMonitoringStore = defineStore('third-party-monitoring', () => {
  const platform = ref<ThirdPartyPlatform>('bilibili')
  const monitors = ref<ThirdPartyMonitorsResponse | null>(null)
  const bilibiliStatus = ref<BilibiliSourceStatusResponse | null>(null)
  const loading = ref(false)
  const restarting = ref(false)
  const error = ref<string | null>(null)

  const items = computed<ThirdPartyMonitorItem[]>(() => monitors.value?.items ?? [])

  async function fetchAll() {
    loading.value = true
    error.value = null
    try {
      const [monitorResponse, statusResponse] = await Promise.all([
        apiRequest<ThirdPartyMonitorsResponse>(`/api/third-party/monitors?platform=${encodeURIComponent(platform.value)}`),
        apiRequest<BilibiliSourceStatusResponse>('/api/bilibili/source/status'),
      ])
      monitors.value = monitorResponse
      bilibiliStatus.value = statusResponse
    } catch (err) {
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      loading.value = false
    }
  }

  async function restartBilibiliSource() {
    restarting.value = true
    try {
      const response = await apiRequest<BilibiliSourceRestartResponse>('/api/bilibili/source/restart', {
        method: 'POST',
      })
      bilibiliStatus.value = response.status
      await fetchAll()
      return response
    } finally {
      restarting.value = false
    }
  }

  return {
    bilibiliStatus,
    error,
    items,
    loading,
    monitors,
    platform,
    restarting,
    fetchAll,
    restartBilibiliSource,
  }
})
