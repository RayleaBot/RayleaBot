import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type {
  PluginDetail,
  PluginDetailResponse,
  PluginGrantListResponse,
  PluginGrantRequest,
  PluginGrantSummary,
  PluginInstallRequest,
  PluginListResponse,
  PluginSettingsResponse,
  PluginSettingsUpdateRequest,
  PluginSettingsUpdateResponse,
  PluginSecretsResponse,
  PluginSecretsUpdateRequest,
  PluginSecretsUpdateResponse,
  PluginSummary,
  TaskAcceptedResponse,
} from '@/types/api'

type PluginUpsert = Partial<PluginSummary> & Pick<PluginSummary, 'id' | 'registration_state' | 'desired_state' | 'runtime_state' | 'display_state'>

export const usePluginsStore = defineStore('plugins', () => {
  const items = ref<PluginSummary[]>([])
  const current = ref<PluginDetail | null>(null)
  const grants = ref<Record<string, PluginGrantSummary[]>>({})
  const settingsByPluginId = ref<Record<string, Record<string, unknown>>>({})
  const secretsByPluginId = ref<Record<string, Record<string, string>>>({})
  const loading = ref(false)
  const detailLoading = ref(false)
  const error = ref<string | null>(null)
  const actionPending = ref<Record<string, string | null>>({})
  const grantsLoading = ref<Record<string, boolean>>({})
  const settingsLoading = ref<Record<string, boolean>>({})
  const settingsSaving = ref<Record<string, boolean>>({})
  const secretsLoading = ref<Record<string, boolean>>({})
  const secretsSaving = ref<Record<string, boolean>>({})
  const installPending = ref(false)
  let detailRequestVersion = 0

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
    detailRequestVersion += 1
    const requestVersion = detailRequestVersion
    try {
      const response = await apiRequest<PluginDetailResponse>(`/api/plugins/${pluginId}`)
      if (requestVersion !== detailRequestVersion) {
        return response.plugin
      }

      current.value = response.plugin
      upsert(response.plugin)
      return response.plugin
    } finally {
      if (requestVersion === detailRequestVersion) {
        detailLoading.value = false
      }
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
      commands: plugin.commands ?? previous?.commands ?? [],
      help: plugin.help ?? previous?.help,
      command_conflicts: plugin.command_conflicts ?? previous?.command_conflicts ?? [],
    }

    if (index === -1) {
      items.value = [...items.value, nextPlugin]
    } else {
      items.value = items.value.map((item, itemIndex) => (itemIndex === index ? nextPlugin : item))
    }

    if (current.value?.id === plugin.id) {
      current.value = {
        ...current.value,
        ...nextPlugin,
      }
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

  function setSettingsLoading(pluginId: string, loadingValue: boolean) {
    settingsLoading.value = {
      ...settingsLoading.value,
      [pluginId]: loadingValue,
    }
  }

  function setSettingsSaving(pluginId: string, loadingValue: boolean) {
    settingsSaving.value = {
      ...settingsSaving.value,
      [pluginId]: loadingValue,
    }
  }

  function setSecretsLoading(pluginId: string, loadingValue: boolean) {
    secretsLoading.value = {
      ...secretsLoading.value,
      [pluginId]: loadingValue,
    }
  }

  function setSecretsSaving(pluginId: string, loadingValue: boolean) {
    secretsSaving.value = {
      ...secretsSaving.value,
      [pluginId]: loadingValue,
    }
  }

  async function executeAction(pluginId: string, action: 'enable' | 'disable' | 'reload') {
    setPending(pluginId, action)
    try {
      const response = await apiRequest<PluginDetailResponse>(`/api/plugins/${pluginId}/${action}`, {
        method: 'POST',
      })
      if (current.value?.id === pluginId) {
        current.value = response.plugin
      }
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

  async function fetchSettings(pluginId: string) {
    setSettingsLoading(pluginId, true)
    try {
      const response = await apiRequest<PluginSettingsResponse>(`/api/plugins/${pluginId}/settings`)
      settingsByPluginId.value = {
        ...settingsByPluginId.value,
        [pluginId]: response.values,
      }
      return response
    } finally {
      setSettingsLoading(pluginId, false)
    }
  }

  async function updateSettings(pluginId: string, values: PluginSettingsUpdateRequest['values']) {
    setSettingsSaving(pluginId, true)
    try {
      const response = await apiRequest<PluginSettingsUpdateResponse>(`/api/plugins/${pluginId}/settings`, {
        method: 'PUT',
        body: {
          values,
        } satisfies PluginSettingsUpdateRequest,
      })
      settingsByPluginId.value = {
        ...settingsByPluginId.value,
        [pluginId]: response.values,
      }
      return response
    } finally {
      setSettingsSaving(pluginId, false)
    }
  }

  async function grantCapability(pluginId: string, payload: PluginGrantRequest) {
    setGrantsLoading(pluginId, true)
    try {
      const response = await apiRequest<PluginGrantSummary>(`/api/plugins/${pluginId}/grants`, {
        method: 'POST',
        body: payload,
      })
      await syncPermissionState(pluginId)
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
      await syncPermissionState(pluginId)
    } finally {
      setGrantsLoading(pluginId, false)
    }
  }

  function getGrants(pluginId: string) {
    return grants.value[pluginId] ?? []
  }

  async function fetchSecrets(pluginId: string) {
    setSecretsLoading(pluginId, true)
    try {
      const response = await apiRequest<PluginSecretsResponse>(`/api/plugins/${pluginId}/secrets`)
      secretsByPluginId.value = {
        ...secretsByPluginId.value,
        [pluginId]: response.values,
      }
      return response
    } finally {
      setSecretsLoading(pluginId, false)
    }
  }

  async function updateSecrets(pluginId: string, values: PluginSecretsUpdateRequest['values'], deletedKeys: string[] = []) {
    setSecretsSaving(pluginId, true)
    try {
      const response = await apiRequest<PluginSecretsUpdateResponse>(`/api/plugins/${pluginId}/secrets`, {
        method: 'PUT',
        body: {
          values,
          deleted_keys: deletedKeys,
        } satisfies PluginSecretsUpdateRequest,
      })
      secretsByPluginId.value = {
        ...secretsByPluginId.value,
        [pluginId]: response.values,
      }
      return response
    } finally {
      setSecretsSaving(pluginId, false)
    }
  }

  function getSettings(pluginId: string) {
    return settingsByPluginId.value[pluginId] ?? {}
  }

  function getSecrets(pluginId: string) {
    return secretsByPluginId.value[pluginId] ?? {}
  }

  async function syncPermissionState(pluginId: string) {
    await fetchGrants(pluginId)
    if (current.value?.id === pluginId) {
      await fetchDetail(pluginId)
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
    settingsByPluginId,
    secretsByPluginId,
    settingsLoading,
    settingsSaving,
    secretsLoading,
    secretsSaving,
    sortedItems,
    executeAction,
    fetchDetail,
    fetchSettings,
    fetchSecrets,
    fetchGrants,
    fetchList,
    getGrants,
    getSettings,
    getSecrets,
    grantCapability,
    installPlugin,
    revokeGrant,
    uninstallPlugin,
    updateSettings,
    updateSecrets,
    upsert,
  }
})
