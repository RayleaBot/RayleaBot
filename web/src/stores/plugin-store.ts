import { ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type {
  PluginStoreDetailResponse,
  PluginStoreEntry,
  PluginStoreInspectionRequest,
  PluginStoreInspectionResponse,
  PluginStoreInstallRequest,
  PluginStoreListResponse,
  PluginStoreSource,
  PluginStoreSourceInput,
  PluginStoreSourcesResponse,
  TaskAcceptedResponse,
} from '@/types/api'

export type PluginStoreSort = 'recommended' | 'name' | 'updated'

export const usePluginStore = defineStore('plugin-store', () => {
  const items = ref<PluginStoreEntry[]>([])
  const source = ref<PluginStoreSource | null>(null)
  const sources = ref<PluginStoreSource[]>([])
  const total = ref(0)
  const loading = ref(false)
  const refreshing = ref(false)
  const sourceSaving = ref(false)
  const installing = ref<Record<string, boolean>>({})
  const error = ref<string | null>(null)

  async function fetchSources() {
    const response = await apiRequest<PluginStoreSourcesResponse>('/api/plugin-store/sources')
    sources.value = response.items
    return response.items
  }

  async function fetchEntries(options: { sourceId?: string; query?: string; sort?: PluginStoreSort } = {}) {
    loading.value = true
    error.value = null
    try {
      const params = new URLSearchParams()
      params.set('source_id', options.sourceId?.trim() || 'official')
      const query = options.query?.trim()
      if (query) params.set('query', query)
      params.set('sort', options.sort ?? 'recommended')
      params.set('limit', '100')
      const response = await apiRequest<PluginStoreListResponse>(`/api/plugin-store/plugins?${params}`)
      items.value = response.items
      total.value = response.total
      source.value = response.source
      updateSource(response.source)
      return response
    } catch (cause) {
      error.value = getDisplayErrorMessage(cause, 'errors.common.loadFailed')
      throw cause
    } finally {
      loading.value = false
    }
  }

  async function fetchDetail(pluginId: string, sourceId = 'official') {
    const params = new URLSearchParams({ source_id: sourceId })
    return await apiRequest<PluginStoreDetailResponse>(`/api/plugin-store/plugins/${encodeURIComponent(pluginId)}?${params}`)
  }

  async function inspect(pluginId: string, payload: PluginStoreInspectionRequest) {
    installing.value = { ...installing.value, [pluginId]: true }
    try {
      return await apiRequest<PluginStoreInspectionResponse>(
        `/api/plugin-store/plugins/${encodeURIComponent(pluginId)}/inspect`,
        { method: 'POST', body: payload },
      )
    } catch (cause) {
      installing.value = { ...installing.value, [pluginId]: false }
      throw cause
    }
  }

  async function install(pluginId: string, payload: PluginStoreInstallRequest) {
    installing.value = { ...installing.value, [pluginId]: true }
    try {
      return await apiRequest<TaskAcceptedResponse>(
        `/api/plugin-store/plugins/${encodeURIComponent(pluginId)}/install`,
        { method: 'POST', body: payload },
      )
    } finally {
      installing.value = { ...installing.value, [pluginId]: false }
    }
  }

  function finishInspection(pluginId: string) {
    installing.value = { ...installing.value, [pluginId]: false }
  }

  async function refreshSource(sourceId: string) {
    refreshing.value = true
    try {
      const response = await apiRequest<PluginStoreSource>(
        `/api/plugin-store/sources/${encodeURIComponent(sourceId)}/refresh`,
        { method: 'POST' },
      )
      source.value = response
      updateSource(response)
      return response
    } finally {
      refreshing.value = false
    }
  }

  async function createSource(input: PluginStoreSourceInput) {
    sourceSaving.value = true
    try {
      const response = await apiRequest<PluginStoreSource>('/api/plugin-store/sources', { method: 'POST', body: input })
      updateSource(response)
      return response
    } finally {
      sourceSaving.value = false
    }
  }

  async function saveSource(sourceId: string, input: PluginStoreSourceInput) {
    sourceSaving.value = true
    try {
      const response = await apiRequest<PluginStoreSource>(
        `/api/plugin-store/sources/${encodeURIComponent(sourceId)}`,
        { method: 'PUT', body: input },
      )
      updateSource(response)
      return response
    } finally {
      sourceSaving.value = false
    }
  }

  async function deleteSource(sourceId: string) {
    sourceSaving.value = true
    try {
      await apiRequest<void>(`/api/plugin-store/sources/${encodeURIComponent(sourceId)}`, { method: 'DELETE' })
      sources.value = sources.value.filter(item => item.id !== sourceId)
    } finally {
      sourceSaving.value = false
    }
  }

  function updateSource(next: PluginStoreSource) {
    const index = sources.value.findIndex(item => item.id === next.id)
    if (index < 0) {
      sources.value = [...sources.value, next]
      return
    }
    sources.value = sources.value.map(item => item.id === next.id ? next : item)
  }

  return {
    error,
    installing,
    items,
    loading,
    refreshing,
    source,
    sourceSaving,
    sources,
    total,
    createSource,
    deleteSource,
    fetchDetail,
    fetchEntries,
    fetchSources,
    finishInspection,
    inspect,
    install,
    refreshSource,
    saveSource,
  }
})
