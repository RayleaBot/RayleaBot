import { onScopeDispose, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import { collectionURL, createCollectionPager, mergeCollectionItems, type CollectionQuery } from '@/lib/collection-pager'
import { waitForTask } from '@/lib/tasks'
import { usePluginsStore } from '@/stores/plugins'
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
type EntryQuery = { sourceId?: string; query?: string; sort?: PluginStoreSort }

export const usePluginStore = defineStore('plugin-store', () => {
  const items = ref<PluginStoreEntry[]>([])
  const source = ref<PluginStoreSource | null>(null)
  const sources = ref<PluginStoreSource[]>([])
  const total = ref(0)
  const loading = ref(false)
  const loadingMore = ref(false)
  const nextCursor = ref('')
  const iconRevision = ref(0)
  const refreshing = ref(false)
  const sourceSaving = ref(false)
  const installing = ref<Record<string, boolean>>({})
  const error = ref<string | null>(null)
  let lastQuery: EntryQuery | null = null
  let entriesRequest = 0
  const installRequests = new Map<string, Promise<TaskAcceptedResponse>>()
  const controllers = new Set<AbortController>()
  onScopeDispose(() => { for (const controller of controllers) controller.abort() })

  const sourcesPager = createCollectionPager<PluginStoreSourcesResponse>({
    request: (query, cursor, signal) => apiRequest(collectionURL('/api/plugin-store/sources', query, cursor), { signal }),
    apply: (response, append) => { sources.value = append ? mergeCollectionItems(sources.value, response.items, item => item.id) : response.items },
  })
  const { total: sourcesTotal, nextCursor: sourcesNextCursor, loading: sourcesLoading, loadingMore: sourcesLoadingMore, error: sourcesError } = sourcesPager
  async function fetchSources(query?: CollectionQuery) { await sourcesPager.load(query); return sources.value }
  function loadMoreSources() { return sourcesPager.loadMore() }

  async function fetchEntries(options: EntryQuery = {}, append = false) {
    if (append && (!nextCursor.value || loading.value || loadingMore.value)) return
    const request = ++entriesRequest
    const cursor = append ? nextCursor.value : ''
    const queryChanged = !lastQuery
      || (lastQuery.sourceId || 'official') !== (options.sourceId || 'official')
      || (lastQuery.query?.trim() || '') !== (options.query?.trim() || '')
      || (lastQuery.sort || 'recommended') !== (options.sort || 'recommended')
    lastQuery = { ...options }
    if (append) loadingMore.value = true
    else {
      loading.value = true
      loadingMore.value = false
      nextCursor.value = ''
      if (queryChanged) { items.value = []; total.value = 0; source.value = null }
    }
    error.value = null
    try {
      const params = new URLSearchParams()
      params.set('source_id', options.sourceId?.trim() || 'official')
      const query = options.query?.trim()
      if (query) params.set('query', query)
      params.set('sort', options.sort ?? 'recommended')
      params.set('limit', '100')
      if (cursor) params.set('cursor', cursor)
      const response = await apiRequest<PluginStoreListResponse>(`/api/plugin-store/plugins?${params}`)
      if (request !== entriesRequest) return response
      items.value = append ? [...new Map([...items.value, ...response.items].map(item => [item.id, item])).values()] : response.items
      nextCursor.value = response.next_cursor || ''
      if (!append) iconRevision.value += 1
      total.value = response.total
      source.value = response.source
      updateSource(response.source)
      return response
    } catch (cause) {
      if (request === entriesRequest) error.value = getDisplayErrorMessage(cause, 'errors.common.loadFailed')
      throw cause
    } finally {
      if (request === entriesRequest) { loading.value = false; loadingMore.value = false }
    }
  }

  async function loadMore() {
    if (lastQuery) return fetchEntries(lastQuery, true)
  }

  async function refreshEntries() {
    if (lastQuery) return fetchEntries(lastQuery)
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

  function install(pluginId: string, payload: PluginStoreInstallRequest): Promise<TaskAcceptedResponse> {
    const pending = installRequests.get(pluginId)
    if (pending) return pending
    installing.value = { ...installing.value, [pluginId]: true }
    const controller = new AbortController()
    controllers.add(controller)
    const request = (async () => {
      try {
        const accepted = await apiRequest<TaskAcceptedResponse>(
        `/api/plugin-store/plugins/${encodeURIComponent(pluginId)}/install`,
          { method: 'POST', body: payload, signal: controller.signal },
        )
        try {
          await waitForTask(accepted.task_id, controller.signal)
        } finally {
          if (!controller.signal.aborted) await Promise.allSettled([usePluginsStore().refreshList(), refreshEntries()])
        }
        return accepted
      } finally {
        controllers.delete(controller)
        installRequests.delete(pluginId)
        installing.value = { ...installing.value, [pluginId]: false }
      }
    })()
    installRequests.set(pluginId, request)
    return request
  }

  function finishInspection(pluginId: string) {
    if (!installRequests.has(pluginId)) installing.value = { ...installing.value, [pluginId]: false }
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
      await fetchSources()
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
      await fetchSources()
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
      await fetchSources()
      return response
    } finally {
      sourceSaving.value = false
    }
  }

  async function deleteSource(sourceId: string) {
    sourceSaving.value = true
    try {
      await apiRequest<void>(`/api/plugin-store/sources/${encodeURIComponent(sourceId)}`, { method: 'DELETE' })
      await fetchSources()
    } finally {
      sourceSaving.value = false
    }
  }

  function updateSource(next: PluginStoreSource) {
    const index = sources.value.findIndex(item => item.id === next.id)
    if (index < 0) {
      return
    }
    sources.value = sources.value.map(item => item.id === next.id ? next : item)
  }

  return {
    sourcesTotal, sourcesNextCursor, sourcesLoading, sourcesLoadingMore, sourcesError, loadMoreSources,
    error,
    installing,
    items,
    loading,
    loadingMore,
    nextCursor,
    iconRevision,
    refreshing,
    source,
    sourceSaving,
    sources,
    total,
    createSource,
    deleteSource,
    fetchDetail,
    fetchEntries,
    loadMore,
    refreshEntries,
    fetchSources,
    finishInspection,
    inspect,
    install,
    refreshSource,
    saveSource,
  }
})
