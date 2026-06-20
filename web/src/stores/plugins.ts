import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type {
  PluginDetail,
  PluginDetailResponse,
  PluginInstallRequest,
  PluginListResponse,
  PluginState,
  PluginSettingsResponse,
  PluginSettingsUpdateRequest,
  PluginSettingsUpdateResponse,
  PluginSecretsResponse,
  PluginSecretsUpdateRequest,
  PluginSecretsUpdateResponse,
  PluginSummary,
  TaskAcceptedResponse,
} from '@/types/api'

type PluginUpsert = Partial<PluginSummary> & Pick<PluginSummary, 'id' | 'state'>

const lifecycleRefreshDelaysMs = [700, 1_500, 3_000, 5_000]

export const usePluginsStore = defineStore('plugins', () => {
  const items = ref<PluginSummary[]>([])
  const current = ref<PluginDetail | null>(null)
  const settingsByPluginId = ref<Record<string, Record<string, unknown>>>({})
  const secretsByPluginId = ref<Record<string, Record<string, string>>>({})
  const loading = ref(false)
  const detailLoading = ref(false)
  const error = ref<string | null>(null)
  const actionPending = ref<Record<string, string | null>>({})
  const settingsLoading = ref<Record<string, boolean>>({})
  const settingsSaving = ref<Record<string, boolean>>({})
  const secretsLoading = ref<Record<string, boolean>>({})
  const secretsSaving = ref<Record<string, boolean>>({})
  const installPending = ref(false)
  let detailRequestVersion = 0
  let listRequest: Promise<void> | null = null
  const lifecycleRefreshTimers = new Map<string, ReturnType<typeof setTimeout>>()
  const lifecycleRefreshAttempts = new Map<string, number>()

  const sortedItems = computed(() => [...items.value].sort((left, right) => left.id.localeCompare(right.id)))

  async function fetchList() {
    if (listRequest) {
      return listRequest
    }

    loading.value = true
    error.value = null
    listRequest = (async () => {
      try {
        const response = await apiRequest<PluginListResponse>('/api/plugins')
        items.value = response.items
        reconcileLifecycleRefreshes(response.items)
      } catch (err) {
        error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
        throw err
      } finally {
        loading.value = false
        listRequest = null
      }
    })()

    return listRequest
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
      updateLifecycleRefresh(pluginId, response.plugin.state)
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
      version: plugin.version ?? previous?.version,
      description: plugin.description ?? previous?.description,
      author: plugin.author ?? previous?.author,
      role: plugin.role ?? previous?.role ?? 'user',
      state: plugin.state,
      state_diagnosis: plugin.state_diagnosis,
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

    if (
      !isLifecycleTransitionState(plugin.state) ||
      lifecycleRefreshTimers.has(plugin.id) ||
      lifecycleRefreshAttempts.has(plugin.id)
    ) {
      updateLifecycleRefresh(plugin.id, plugin.state)
    }
  }

  function isLifecycleTransitionState(state?: PluginState | string) {
    return state === 'starting' || state === 'stopping'
  }

  function clearLifecycleRefresh(pluginId: string) {
    const timer = lifecycleRefreshTimers.get(pluginId)
    if (timer) {
      clearTimeout(timer)
    }
    lifecycleRefreshTimers.delete(pluginId)
    lifecycleRefreshAttempts.delete(pluginId)
  }

  function getKnownPluginState(pluginId: string) {
    return items.value.find((item) => item.id === pluginId)?.state ?? (
      current.value?.id === pluginId ? current.value.state : undefined
    )
  }

  async function refreshPluginStateFromServer(pluginId: string) {
    if (current.value?.id === pluginId) {
      await fetchDetail(pluginId)
      return
    }

    await fetchList()
  }

  function updateLifecycleRefresh(pluginId: string, state?: PluginState | string) {
    if (!isLifecycleTransitionState(state)) {
      clearLifecycleRefresh(pluginId)
      return
    }

    if (lifecycleRefreshTimers.has(pluginId)) {
      return
    }

    const attempt = lifecycleRefreshAttempts.get(pluginId) ?? 0
    if (attempt >= lifecycleRefreshDelaysMs.length) {
      return
    }

    lifecycleRefreshAttempts.set(pluginId, attempt + 1)
    lifecycleRefreshTimers.set(pluginId, setTimeout(() => {
      lifecycleRefreshTimers.delete(pluginId)
      void refreshPluginStateFromServer(pluginId)
        .catch(() => undefined)
        .finally(() => {
          updateLifecycleRefresh(pluginId, getKnownPluginState(pluginId))
        })
    }, lifecycleRefreshDelaysMs[attempt]))
  }

  function reconcileLifecycleRefreshes(nextItems: PluginSummary[]) {
    const seenPluginIds = new Set<string>()

    for (const item of nextItems) {
      seenPluginIds.add(item.id)
      updateLifecycleRefresh(item.id, item.state)
    }

    const trackedPluginIds = new Set([
      ...lifecycleRefreshTimers.keys(),
      ...lifecycleRefreshAttempts.keys(),
    ])
    for (const pluginId of trackedPluginIds) {
      if (!seenPluginIds.has(pluginId) && current.value?.id !== pluginId) {
        clearLifecycleRefresh(pluginId)
      }
    }
  }

  function setPending(pluginId: string, action: string | null) {
    actionPending.value = {
      ...actionPending.value,
      [pluginId]: action,
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
      updateLifecycleRefresh(pluginId, response.plugin.state)
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

  return {
    actionPending,
    current,
    detailLoading,
    error,
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
    fetchList,
    getSettings,
    getSecrets,
    installPlugin,
    uninstallPlugin,
    updateSettings,
    updateSecrets,
    upsert,
  }
})
