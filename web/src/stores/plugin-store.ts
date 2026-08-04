import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type {
  PluginStoreCatalogStatus,
  PluginStoreDetailResponse,
  PluginStoreEntry,
  PluginStoreInstallRequest,
  PluginStoreListResponse,
  PluginStoreRefreshResponse,
  TaskAcceptedResponse,
} from '@/types/api'

export type PluginStoreSort = 'recommended' | 'name' | 'updated'

export const usePluginStore = defineStore('plugin-store', () => {
  const items = ref<PluginStoreEntry[]>([])
  const catalog = ref<PluginStoreCatalogStatus | null>(null)
  const total = ref(0)
  const loading = ref(false)
  const refreshing = ref(false)
  const installing = ref<Record<string, boolean>>({})
  const error = ref<string | null>(null)

  const hasVerifiedCatalog = computed(() => catalog.value?.verified === true)

  async function fetchEntries(options: { query?: string; sort?: PluginStoreSort } = {}) {
    loading.value = true
    error.value = null
    try {
      const params = new URLSearchParams()
      const query = options.query?.trim()
      if (query) params.set('query', query)
      params.set('sort', options.sort ?? 'recommended')
      params.set('limit', '100')
      const response = await apiRequest<PluginStoreListResponse>(`/api/plugin-store/plugins?${params}`)
      items.value = response.items
      total.value = response.total
      catalog.value = response.catalog
      return response
    } catch (cause) {
      error.value = getDisplayErrorMessage(cause, 'errors.common.loadFailed')
      throw cause
    } finally {
      loading.value = false
    }
  }

  async function fetchDetail(pluginId: string) {
    return await apiRequest<PluginStoreDetailResponse>(`/api/plugin-store/plugins/${encodeURIComponent(pluginId)}`)
  }

  async function install(pluginId: string, version?: string) {
    installing.value = { ...installing.value, [pluginId]: true }
    try {
      const body: PluginStoreInstallRequest = {
        trusted_code_confirmed: true,
        ...(version ? { version } : {}),
      }
      return await apiRequest<TaskAcceptedResponse>(
        `/api/plugin-store/plugins/${encodeURIComponent(pluginId)}/install`,
        { method: 'POST', body },
      )
    } finally {
      installing.value = { ...installing.value, [pluginId]: false }
    }
  }

  async function refreshCatalog() {
    refreshing.value = true
    try {
      const response = await apiRequest<PluginStoreRefreshResponse>('/api/plugin-store/refresh', { method: 'POST' })
      catalog.value = response.catalog
      await fetchEntries()
      return response.catalog
    } finally {
      refreshing.value = false
    }
  }

  return {
    catalog,
    error,
    hasVerifiedCatalog,
    installing,
    items,
    loading,
    refreshing,
    total,
    fetchDetail,
    fetchEntries,
    install,
    refreshCatalog,
  }
})
