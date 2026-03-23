import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { apiRequest } from '@/lib/http'
import type { PluginDetailResponse, PluginListResponse, PluginSummary } from '@/types/api'

export interface ConsoleFrame {
  plugin_id: string
  stream: 'stdout' | 'stderr' | 'system'
  text: string
  timestamp: string
}

export const usePluginsStore = defineStore('plugins', () => {
  const items = ref<PluginSummary[]>([])
  const current = ref<PluginSummary | null>(null)
  const consoleFrames = ref<Record<string, ConsoleFrame[]>>({})
  const loading = ref(false)
  const error = ref<string | null>(null)
  const actionPending = ref<Record<string, string | null>>({})

  const sortedItems = computed(() => [...items.value].sort((left, right) => left.id.localeCompare(right.id)))

  async function fetchList() {
    loading.value = true
    error.value = null
    try {
      const response = await apiRequest<PluginListResponse>('/api/plugins')
      items.value = response.items
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'plugin list failed'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function fetchDetail(pluginId: string) {
    const response = await apiRequest<PluginDetailResponse>(`/api/plugins/${pluginId}`)
    current.value = response.plugin
    upsert(response.plugin)
    return response.plugin
  }

  function upsert(plugin: PluginSummary) {
    const index = items.value.findIndex((item) => item.id === plugin.id)
    if (index === -1) {
      items.value = [...items.value, plugin]
    } else {
      items.value = items.value.map((item, itemIndex) => (itemIndex === index ? plugin : item))
    }

    if (current.value?.id === plugin.id) {
      current.value = plugin
    }
  }

  function setPending(pluginId: string, action: string | null) {
    actionPending.value = {
      ...actionPending.value,
      [pluginId]: action,
    }
  }

  async function executeAction(pluginId: string, action: 'enable' | 'disable' | 'reload') {
    setPending(pluginId, action)
    try {
      const response = await apiRequest<PluginDetailResponse>(`/api/plugins/${pluginId}/${action}`, {
        method: 'POST',
      })
      upsert(response.plugin)
      return response.plugin
    } finally {
      setPending(pluginId, null)
    }
  }

  function appendConsole(frame: ConsoleFrame) {
    const existing = consoleFrames.value[frame.plugin_id] ?? []
    consoleFrames.value = {
      ...consoleFrames.value,
      [frame.plugin_id]: [...existing, frame].slice(-100),
    }
  }

  function getConsole(pluginId: string) {
    return consoleFrames.value[pluginId] ?? []
  }

  return {
    actionPending,
    current,
    error,
    items,
    loading,
    sortedItems,
    appendConsole,
    executeAction,
    fetchDetail,
    fetchList,
    getConsole,
    upsert,
  }
})
