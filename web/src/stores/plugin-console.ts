import { ref } from 'vue'
import { defineStore } from 'pinia'

import { apiRequest } from '@/lib/http'
import type {
  LogListResponse,
  LogSummary,
  PluginConsoleFrameData,
} from '@/types/api'

export type ProcessConsoleFrame = PluginConsoleFrameData

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

export const usePluginConsoleStore = defineStore('plugin-console', () => {
  const processConsoleFrames = ref<Record<string, ProcessConsoleFrame[]>>({})
  const outboundConsoleFrames = ref<Record<string, OutboundConsoleFrame[]>>({})

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

  return {
    appendConsole,
    appendOutboundLog,
    clearConsole,
    fetchOutboundConsoleHistory,
    getConsole,
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
