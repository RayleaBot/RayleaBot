import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type {
  LogListResponse,
  LogSummary,
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
  PluginSummary,
  TaskAcceptedResponse,
} from '@/types/api'

type PluginUpsert = Partial<PluginSummary> & Pick<PluginSummary, 'id' | 'registration_state' | 'desired_state' | 'runtime_state' | 'display_state'>

export interface ProcessConsoleFrame {
  plugin_id: string
  stream: 'stdout' | 'stderr' | 'system'
  text: string
  timestamp: string
}

export interface OutboundConsoleFrame {
  log_id: string
  plugin_id: string
  stream: 'outbound'
  level: NonNullable<LogSummary['level']>
  text: string
  timestamp: string
  request_id?: string
}

export type ConsoleFrame = ProcessConsoleFrame | OutboundConsoleFrame

export const usePluginsStore = defineStore('plugins', () => {
  const items = ref<PluginSummary[]>([])
  const current = ref<PluginDetail | null>(null)
  const grants = ref<Record<string, PluginGrantSummary[]>>({})
  const processConsoleFrames = ref<Record<string, ProcessConsoleFrame[]>>({})
  const outboundConsoleFrames = ref<Record<string, OutboundConsoleFrame[]>>({})
  const settingsByPluginId = ref<Record<string, Record<string, unknown>>>({})
  const loading = ref(false)
  const detailLoading = ref(false)
  const error = ref<string | null>(null)
  const actionPending = ref<Record<string, string | null>>({})
  const grantsLoading = ref<Record<string, boolean>>({})
  const settingsLoading = ref<Record<string, boolean>>({})
  const settingsSaving = ref<Record<string, boolean>>({})
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
      command_conflicts: plugin.command_conflicts ?? previous?.command_conflicts,
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

  function mergeOutboundFrames(pluginId: string, frames: OutboundConsoleFrame[]) {
    if (frames.length === 0) {
      return
    }

    const existing = outboundConsoleFrames.value[pluginId] ?? []
    outboundConsoleFrames.value = {
      ...outboundConsoleFrames.value,
      [pluginId]: mergeOutboundBuffer(existing, frames),
    }
  }

  async function fetchOutboundConsoleHistory(pluginId: string) {
    const normalizedPluginId = normalizePluginID(pluginId)
    if (!normalizedPluginId) {
      return []
    }

    const params = new URLSearchParams({
      plugin_id: normalizedPluginId,
      source: 'adapter.onebot11',
      limit: '100',
    })
    const response = await apiRequest<LogListResponse>(`/api/logs?${params.toString()}`)
    const frames = response.items
      .map((item) => toOutboundConsoleFrame(item))
      .filter((item): item is OutboundConsoleFrame => item !== null && item.plugin_id === normalizedPluginId)
    mergeOutboundFrames(normalizedPluginId, frames)
    return outboundConsoleFrames.value[normalizedPluginId] ?? []
  }

  function appendConsole(frame: ProcessConsoleFrame) {
    const normalizedPluginId = normalizePluginID(frame.plugin_id)
    if (!normalizedPluginId) {
      return
    }

    const existing = processConsoleFrames.value[normalizedPluginId] ?? []
    processConsoleFrames.value = {
      ...processConsoleFrames.value,
      [normalizedPluginId]: [...existing, { ...frame, plugin_id: normalizedPluginId }].slice(-100),
    }
  }

  function appendOutboundLog(log: LogSummary) {
    const frame = toOutboundConsoleFrame(log)
    if (!frame) {
      return
    }

    mergeOutboundFrames(frame.plugin_id, [frame])
  }

  function getConsole(pluginId: string) {
    const normalizedPluginId = normalizePluginID(pluginId)
    if (!normalizedPluginId) {
      return []
    }

    const processFrames = processConsoleFrames.value[normalizedPluginId] ?? []
    const outboundFrames = outboundConsoleFrames.value[normalizedPluginId] ?? []
    return [...processFrames, ...outboundFrames]
      .sort(compareConsoleFrames)
      .slice(-200)
  }

  function getGrants(pluginId: string) {
    return grants.value[pluginId] ?? []
  }

  function getSettings(pluginId: string) {
    return settingsByPluginId.value[pluginId] ?? {}
  }

  function clearConsole(pluginId: string) {
    const normalizedPluginId = normalizePluginID(pluginId)
    if (!normalizedPluginId) {
      return
    }

    processConsoleFrames.value = {
      ...processConsoleFrames.value,
      [normalizedPluginId]: [],
    }
    outboundConsoleFrames.value = {
      ...outboundConsoleFrames.value,
      [normalizedPluginId]: [],
    }
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
    settingsLoading,
    settingsSaving,
    sortedItems,
    appendConsole,
    clearConsole,
    executeAction,
    fetchDetail,
    fetchSettings,
    fetchOutboundConsoleHistory,
    fetchGrants,
    fetchList,
    getConsole,
    getGrants,
    getSettings,
    grantCapability,
    installPlugin,
    appendOutboundLog,
    revokeGrant,
    uninstallPlugin,
    updateSettings,
    upsert,
  }
})

function mergeOutboundBuffer(existing: OutboundConsoleFrame[], incoming: OutboundConsoleFrame[]) {
  const nextItems = new Map<string, OutboundConsoleFrame>()
  for (const item of [...existing, ...incoming]) {
    nextItems.set(item.log_id, item)
  }
  return Array.from(nextItems.values())
    .sort(compareConsoleFrames)
    .slice(-100)
}

function toOutboundConsoleFrame(log: LogSummary): OutboundConsoleFrame | null {
  const pluginId = normalizePluginID(log.plugin_id)
  if (!pluginId || log.source !== 'adapter.onebot11') {
    return null
  }

  return {
    log_id: log.log_id,
    plugin_id: pluginId,
    stream: 'outbound',
    level: (log.level ?? 'info') as OutboundConsoleFrame['level'],
    text: log.message,
    timestamp: log.timestamp,
    request_id: log.request_id ?? undefined,
  }
}

function compareConsoleFrames(left: ConsoleFrame, right: ConsoleFrame) {
  const timestampCompare = compareConsoleTimestamps(left.timestamp, right.timestamp)
  if (timestampCompare !== 0) {
    return timestampCompare
  }

  if (left.stream !== right.stream) {
    return left.stream.localeCompare(right.stream)
  }

  return getConsoleFrameIdentity(left).localeCompare(getConsoleFrameIdentity(right))
}

function compareConsoleTimestamps(left: string, right: string) {
  const leftValue = toConsoleTimestampValue(left)
  const rightValue = toConsoleTimestampValue(right)
  if (leftValue !== null && rightValue !== null && leftValue !== rightValue) {
    return leftValue - rightValue
  }

  if (left !== right) {
    return left.localeCompare(right)
  }

  return 0
}

function toConsoleTimestampValue(value: string) {
  const trimmed = value.trim()
  if (!trimmed) {
    return null
  }

  const numericValue = Number(trimmed)
  if (Number.isFinite(numericValue)) {
    return normalizeUnixTimestamp(numericValue)
  }

  const parsed = Date.parse(trimmed)
  if (Number.isNaN(parsed)) {
    return null
  }
  return parsed
}

function normalizeUnixTimestamp(value: number) {
  const absolute = Math.abs(value)
  if (absolute >= 1_000_000_000 && absolute < 1_000_000_000_000) {
    return value * 1000
  }
  if (absolute >= 1_000_000_000_000 && absolute <= 8_640_000_000_000_000) {
    return value
  }
  return null
}

function getConsoleFrameIdentity(frame: ConsoleFrame) {
  if (frame.stream === 'outbound') {
    return frame.log_id
  }
  return `${frame.plugin_id}:${frame.stream}:${frame.timestamp}:${frame.text}`
}

function normalizePluginID(pluginId: string | null | undefined) {
  const normalizedPluginId = pluginId?.trim()
  return normalizedPluginId ? normalizedPluginId : ''
}
