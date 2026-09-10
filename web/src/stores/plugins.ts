import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import { waitForTask } from '@/lib/tasks'
import type {
  PluginDetail,
  PluginDetailResponse,
  PluginInstallRequest,
  PluginInstallInspectionRequest,
  PluginInstallInspectionResponse,
  PluginListResponse,
  PluginState,
  PluginSettingsResponse,
  PluginSettingsUpdateRequest,
  PluginSettingsUpdateResponse,
  PluginSummary,
  TaskAcceptedResponse,
} from '@/types/api'

type PluginUpsert = Partial<PluginSummary> & Pick<PluginSummary, 'id' | 'state'>

interface EnsurePluginDetailOptions {
  refresh?: boolean
}

const lifecycleRefreshDelaysMs = [700, 1_500, 3_000, 5_000]

export const usePluginsStore = defineStore('plugins', () => {
  const items = ref<PluginSummary[]>([])
  const current = ref<PluginDetail | null>(null)
  const detailsByPluginId = ref<Record<string, PluginDetail>>({})
  const detailErrorsByPluginId = ref<Record<string, string | null>>({})
  const detailLoadingByPluginId = ref<Record<string, boolean>>({})
  const pluginNameCache = ref<Record<string, string>>({})
  const settingsByPluginId = ref<Record<string, Record<string, unknown>>>({})
  const loading = ref(false)
  const detailLoading = ref(false)
  const error = ref<string | null>(null)
  const actionPending = ref<Record<string, string | null>>({})
  const settingsLoading = ref<Record<string, boolean>>({})
  const settingsSaving = ref<Record<string, boolean>>({})
  const installPending = ref(false)
  const inspectionPending = ref(false)
  const listLoaded = ref(false)
  const iconRevision = ref(0)
  const detailGenerations = new Map<string, number>()
  let detailRequestVersion = 0
  let listRequest: Promise<void> | null = null
  const detailRequests = new Map<string, Promise<PluginDetail>>()
  const lifecycleRefreshTimers = new Map<string, ReturnType<typeof setTimeout>>()
  const lifecycleRefreshAttempts = new Map<string, number>()

  const sortedItems = computed(() => [...items.value].sort((left, right) => left.id.localeCompare(right.id)))

  function rememberPluginNames(plugins: Array<Pick<PluginSummary, 'id' | 'name'>>) {
    const nextNames = { ...pluginNameCache.value }
    let changed = false
    for (const plugin of plugins) {
      const name = plugin.name?.trim()
      if (name && nextNames[plugin.id] !== name) {
        nextNames[plugin.id] = name
        changed = true
      }
    }
    if (changed) {
      pluginNameCache.value = nextNames
    }
  }

  function getPluginDisplayName(pluginId: string, fallback?: string) {
    return items.value.find(item => item.id === pluginId)?.name?.trim() || fallback?.trim() || pluginNameCache.value[pluginId] || pluginId
  }

  function getPluginLabel(pluginId: string, fallback?: string) {
    const name = getPluginDisplayName(pluginId, fallback)
    return name === pluginId ? pluginId : `${name}（${pluginId}）`
  }

  function invalidatePendingDetail(pluginId: string) {
    detailGenerations.set(pluginId, (detailGenerations.get(pluginId) ?? 0) + 1)
  }

  function syncDetailSummary(summary: PluginSummary) {
    const cached = detailsByPluginId.value[summary.id]
    // A new package may also change management pages and other detail-only fields.
    if (cached && cached.version !== summary.version) {
      const next = { ...detailsByPluginId.value }
      delete next[summary.id]
      detailsByPluginId.value = next
    } else if (cached) {
      detailsByPluginId.value = { ...detailsByPluginId.value, [summary.id]: { ...cached, ...summary } }
    }
    if (current.value?.id === summary.id) {
      if (current.value.version !== summary.version) {
        current.value = null
        void fetchDetail(summary.id).catch(() => undefined)
      } else {
        current.value = { ...current.value, ...summary }
      }
    }
  }

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
        iconRevision.value += 1
        const ids = new Set(response.items.map(item => item.id))
        for (const id of new Set([...ids, ...Object.keys(detailsByPluginId.value), ...detailRequests.keys()])) {
          invalidatePendingDetail(id)
        }
        // Development packages can change detail metadata without a version bump.
        detailsByPluginId.value = {}
        const currentId = current.value?.id
        current.value = null
        listLoaded.value = true
        rememberPluginNames(response.items)
        reconcileLifecycleRefreshes(response.items)
        if (currentId && ids.has(currentId)) void fetchDetail(currentId).catch(() => undefined)
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

  async function ensureList() {
    if (listLoaded.value) {
      return
    }

    await fetchList()
  }

  async function refreshList() {
    // A read started before a completed mutation cannot confirm its result.
    if (listRequest) await listRequest.catch(() => undefined)
    await fetchList()
  }

  function setDetailLoading(pluginId: string, loadingValue: boolean) {
    detailLoadingByPluginId.value = {
      ...detailLoadingByPluginId.value,
      [pluginId]: loadingValue,
    }
  }

  function setDetailError(pluginId: string, errorValue: string | null) {
    detailErrorsByPluginId.value = {
      ...detailErrorsByPluginId.value,
      [pluginId]: errorValue,
    }
  }

  function cachePluginDetail(plugin: PluginDetail) {
    detailsByPluginId.value = {
      ...detailsByPluginId.value,
      [plugin.id]: plugin,
    }
    if (current.value?.id === plugin.id) current.value = plugin
    iconRevision.value += 1
    upsert(plugin, true)
    updateLifecycleRefresh(plugin.id, plugin.state)
  }

  function requestPluginDetail(pluginId: string): Promise<PluginDetail> {
    const pendingRequest = detailRequests.get(pluginId)
    if (pendingRequest) {
      return pendingRequest
    }

    setDetailLoading(pluginId, true)
    setDetailError(pluginId, null)
    const generation = detailGenerations.get(pluginId) ?? 0
    let request!: Promise<PluginDetail>
    request = (async () => {
      try {
        const response = await apiRequest<PluginDetailResponse>(`/api/plugins/${pluginId}`)
        if (generation !== (detailGenerations.get(pluginId) ?? 0)) {
          if (listLoaded.value && !items.value.some(item => item.id === pluginId)) throw new Error('插件已不可用')
          if (detailRequests.get(pluginId) === request) detailRequests.delete(pluginId)
          return requestPluginDetail(pluginId)
        }
        cachePluginDetail(response.plugin)
        return response.plugin
      } catch (err) {
        if (generation === (detailGenerations.get(pluginId) ?? 0)) setDetailError(pluginId, getDisplayErrorMessage(err, 'errors.common.loadFailed'))
        throw err
      } finally {
        if (detailRequests.get(pluginId) === request) {
          setDetailLoading(pluginId, false)
          detailRequests.delete(pluginId)
        }
      }
    })()
    detailRequests.set(pluginId, request)
    return request
  }

  async function ensureDetail(pluginId: string, options: EnsurePluginDetailOptions = {}) {
    const cachedDetail = detailsByPluginId.value[pluginId]
    if (cachedDetail && !options.refresh) {
      return cachedDetail
    }

    return requestPluginDetail(pluginId)
  }

  async function fetchDetail(pluginId: string) {
    detailLoading.value = true
    detailRequestVersion += 1
    const requestVersion = detailRequestVersion
    try {
      const plugin = await requestPluginDetail(pluginId)
      if (requestVersion !== detailRequestVersion) {
        return plugin
      }

      current.value = plugin
      return plugin
    } finally {
      if (requestVersion === detailRequestVersion) {
        detailLoading.value = false
      }
    }
  }

  function upsert(plugin: PluginUpsert, fullSnapshot = false) {
    invalidatePendingDetail(plugin.id)
    const index = items.value.findIndex((item) => item.id === plugin.id)
    const previous = fullSnapshot ? null : index === -1 ? current.value?.id === plugin.id ? current.value : null : items.value[index]
    const nextPlugin: PluginSummary = {
      id: plugin.id,
      name: plugin.name ?? previous?.name ?? plugin.id,
      version: plugin.version ?? previous?.version,
      description: plugin.description ?? previous?.description,
      author: plugin.author ?? previous?.author,
      icon: plugin.icon ?? previous?.icon,
      role: plugin.role ?? previous?.role ?? 'community',
      state: plugin.state,
      state_diagnosis: plugin.state_diagnosis,
      source: plugin.source ?? previous?.source,
      trust: plugin.trust ?? previous?.trust,
      commands: plugin.commands ?? previous?.commands ?? [],
    command_groups: plugin.command_groups ?? previous?.command_groups ?? [],
    help: plugin.help ?? previous?.help ?? {},
      command_conflicts: plugin.command_conflicts ?? previous?.command_conflicts ?? [],
    }

    if (index === -1) {
      items.value = [...items.value, nextPlugin]
    } else {
      items.value = items.value.map((item, itemIndex) => (itemIndex === index ? nextPlugin : item))
    }

    syncDetailSummary(nextPlugin)

    rememberPluginNames([nextPlugin])

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

  async function executeAction(pluginId: string, action: 'enable' | 'disable' | 'reload') {
    setPending(pluginId, action)
    try {
      const response = await apiRequest<PluginDetailResponse>(`/api/plugins/${pluginId}/${action}`, {
        method: 'POST',
      })
      cachePluginDetail(response.plugin)
      if (current.value?.id === pluginId) {
        current.value = response.plugin
      }
      return response.plugin
    } finally {
      setPending(pluginId, null)
    }
  }

  async function installPlugin(payload: PluginInstallRequest, onAccepted?: () => void) {
    installPending.value = true
    try {
      const accepted = await apiRequest<TaskAcceptedResponse>('/api/plugins/install', {
        method: 'POST',
        body: payload,
      })
      onAccepted?.()
      try { await waitForTask(accepted.task_id) } finally { await refreshList().catch(() => undefined) }
      return accepted
    } finally {
      installPending.value = false
    }
  }

  async function uninstallPlugin(pluginId: string, onAccepted?: () => void) {
    setPending(pluginId, 'uninstall')
    try {
      const accepted = await apiRequest<TaskAcceptedResponse>(`/api/plugins/${pluginId}`, {
        method: 'DELETE',
      })
      onAccepted?.()
      try { await waitForTask(accepted.task_id) } finally { await refreshList().catch(() => undefined) }
      return accepted
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

  async function inspectPlugin(payload: PluginInstallInspectionRequest) {
    inspectionPending.value = true
    try {
      return await apiRequest<PluginInstallInspectionResponse>('/api/plugins/install/inspect', {
        method: 'POST',
        body: payload,
      })
    } finally {
      inspectionPending.value = false
    }
  }

  function getSettings(pluginId: string) {
    return settingsByPluginId.value[pluginId] ?? {}
  }

  return {
    actionPending,
    current,
    detailErrorsByPluginId,
    detailLoading,
    detailLoadingByPluginId,
    detailsByPluginId,
    error,
    items,
    installPending,
    inspectionPending,
    listLoaded,
    iconRevision,
    loading,
    settingsByPluginId,
    settingsLoading,
    settingsSaving,
    sortedItems,
    executeAction,
    fetchDetail,
    fetchSettings,
    fetchList,
    refreshList,
    ensureDetail,
    ensureList,
    getSettings,
    getPluginDisplayName,
    getPluginLabel,
    installPlugin,
    inspectPlugin,
    uninstallPlugin,
    updateSettings,
    upsert,
  }
})
