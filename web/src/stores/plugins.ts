import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type {
  PluginDetailResponse,
  PluginGrantListResponse,
  PluginGrantRequest,
  PluginGrantSummary,
  PluginInstallRequest,
  PluginListResponse,
  PluginSummary,
  TaskAcceptedResponse,
} from '@/types/api'

type PluginUpsert = Partial<PluginSummary> & Pick<PluginSummary, 'id' | 'registration_state' | 'desired_state' | 'runtime_state'>

export interface ConsoleFrame {
  plugin_id: string
  stream: 'stdout' | 'stderr' | 'system'
  text: string
  timestamp: string
}

export const usePluginsStore = defineStore('plugins', () => {
  const items = ref<PluginSummary[]>([])
  const current = ref<PluginSummary | null>(null)
  const grants = ref<Record<string, PluginGrantSummary[]>>({})
  const consoleFrames = ref<Record<string, ConsoleFrame[]>>({})
  const loading = ref(false)
  const detailLoading = ref(false)
  const error = ref<string | null>(null)
  const actionPending = ref<Record<string, string | null>>({})
  const grantsLoading = ref<Record<string, boolean>>({})
  const installPending = ref(false)

  const sortedItems = computed(() => [...items.value].sort((left, right) => left.id.localeCompare(right.id)))

  async function fetchList() {
    loading.value = true
    error.value = null
    try {
      const response = await apiRequest<PluginListResponse>('/api/plugins')
      items.value = response.items
    } catch (err) {
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      loading.value = false
    }
  }

  async function fetchDetail(pluginId: string) {
    detailLoading.value = true
    try {
      const response = await apiRequest<PluginDetailResponse>(`/api/plugins/${pluginId}`)
      current.value = response.plugin
      upsert(response.plugin)
      return response.plugin
    } finally {
      detailLoading.value = false
    }
  }

  function upsert(plugin: PluginUpsert) {
    const index = items.value.findIndex((item) => item.id === plugin.id)
    const previous = index === -1 ? current.value?.id === plugin.id ? current.value : null : items.value[index]
    const nextPlugin: PluginSummary = {
      id: plugin.id,
      name: plugin.name ?? previous?.name ?? plugin.id,
      role: plugin.role ?? previous?.role ?? 'user',
      registration_state: plugin.registration_state,
      desired_state: plugin.desired_state,
      runtime_state: plugin.runtime_state,
      display_state: plugin.display_state ?? previous?.display_state,
      source: plugin.source ?? previous?.source,
      trust: plugin.trust ?? previous?.trust,
      command_conflicts: plugin.command_conflicts ?? previous?.command_conflicts,
    }

    if (index === -1) {
      items.value = [...items.value, nextPlugin]
    } else {
      items.value = items.value.map((item, itemIndex) => (itemIndex === index ? nextPlugin : item))
    }

    if (current.value?.id === plugin.id) {
      current.value = nextPlugin
    }
  }

  function setPending(pluginId: string, action: string | null) {
    actionPending.value = {
      ...actionPending.value,
      [pluginId]: action,
    }
  }

  function setGrantsLoading(pluginId: string, loadingValue: boolean) {
    grantsLoading.value = {
      ...grantsLoading.value,
      [pluginId]: loadingValue,
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

  async function installPlugin(payload: PluginInstallRequest) {
    installPending.value = true
    try {
      return await apiRequest<TaskAcceptedResponse>('/api/plugins/install', {
        method: 'POST',
        body: payload,
      })
    } finally {
      installPending.value = false
    }
  }

  async function uninstallPlugin(pluginId: string) {
    setPending(pluginId, 'uninstall')
    try {
      return await apiRequest<TaskAcceptedResponse>(`/api/plugins/${pluginId}`, {
        method: 'DELETE',
      })
    } finally {
      setPending(pluginId, null)
    }
  }

  async function fetchGrants(pluginId: string) {
    setGrantsLoading(pluginId, true)
    try {
      const response = await apiRequest<PluginGrantListResponse>(`/api/plugins/${pluginId}/grants`)
      grants.value = {
        ...grants.value,
        [pluginId]: response.items,
      }
      return response.items
    } finally {
      setGrantsLoading(pluginId, false)
    }
  }

  async function grantCapability(pluginId: string, payload: PluginGrantRequest) {
    setGrantsLoading(pluginId, true)
    try {
      const response = await apiRequest<PluginGrantSummary>(`/api/plugins/${pluginId}/grants`, {
        method: 'POST',
        body: payload,
      })
      const nextItems = [...(grants.value[pluginId] ?? []).filter((item) => item.capability !== response.capability), response]
      grants.value = {
        ...grants.value,
        [pluginId]: nextItems.sort((left, right) => left.capability.localeCompare(right.capability)),
      }
      return response
    } finally {
      setGrantsLoading(pluginId, false)
    }
  }

  async function revokeGrant(pluginId: string, capability: string) {
    setGrantsLoading(pluginId, true)
    try {
      await apiRequest<void>(`/api/plugins/${pluginId}/grants/${encodeURIComponent(capability)}`, {
        method: 'DELETE',
      })
      grants.value = {
        ...grants.value,
        [pluginId]: (grants.value[pluginId] ?? []).filter((item) => item.capability !== capability),
      }
    } finally {
      setGrantsLoading(pluginId, false)
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

  function getGrants(pluginId: string) {
    return grants.value[pluginId] ?? []
  }

  function clearConsole(pluginId: string) {
    consoleFrames.value = {
      ...consoleFrames.value,
      [pluginId]: [],
    }
  }

  return {
    actionPending,
    current,
    detailLoading,
    error,
    grants,
    grantsLoading,
    items,
    installPending,
    loading,
    sortedItems,
    appendConsole,
    clearConsole,
    executeAction,
    fetchDetail,
    fetchGrants,
    fetchList,
    getConsole,
    getGrants,
    grantCapability,
    installPlugin,
    revokeGrant,
    uninstallPlugin,
    upsert,
  }
})
