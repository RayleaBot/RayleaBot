import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiDownload, apiRequest } from '@/lib/http'
import type {
  BilibiliSourceRestartResponse,
  BilibiliSourceStatusEventPayload,
  BilibiliSourceStatusResponse,
  ThirdPartyMonitorItem,
  ThirdPartyMonitorsResponse,
  ThirdPartyPlatform,
} from '@/types/api'

const silentRefreshDebounceMs = 120

export const useThirdPartyMonitoringStore = defineStore('third-party-monitoring', () => {
  const platform = ref<ThirdPartyPlatform>('bilibili')
  const monitors = ref<ThirdPartyMonitorsResponse | null>(null)
  const bilibiliStatus = ref<BilibiliSourceStatusResponse | null>(null)
  const loading = ref(false)
  const restarting = ref(false)
  const error = ref<string | null>(null)
  const lastRefreshedAt = ref<string | null>(null)
  const mediaObjectURLs = new Map<string, string>()
  let active = false
  let lastSignature: string | null = null
  let silentRefreshHandle: number | null = null
  let pendingMonitorsRefresh = false
  let silentRefreshInFlight = false
  let silentRefreshQueued = false

  const items = computed<ThirdPartyMonitorItem[]>(() => monitors.value?.items ?? [])

  async function fetchAll() {
    loading.value = true
    error.value = null
    try {
      const [monitorResponse, statusResponse] = await Promise.all([
        apiRequest<ThirdPartyMonitorsResponse>(`/api/third-party/monitors?platform=${encodeURIComponent(platform.value)}`),
        apiRequest<BilibiliSourceStatusResponse>('/api/bilibili/source/status'),
      ])
      monitors.value = await resolveMonitorMedia(monitorResponse)
      bilibiliStatus.value = statusResponse
      lastSignature = signatureFromStatusResponse(statusResponse)
      lastRefreshedAt.value = new Date().toISOString()
    } catch (err) {
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      loading.value = false
    }
  }

  function activate() {
    active = true
  }

  function deactivate() {
    active = false
    lastSignature = null
    pendingMonitorsRefresh = false
    silentRefreshQueued = false
    lastRefreshedAt.value = null
    if (silentRefreshHandle !== null) {
      window.clearTimeout(silentRefreshHandle)
      silentRefreshHandle = null
    }
    disposeMedia()
  }

  function handleSourceStatusEvent(payload: BilibiliSourceStatusEventPayload) {
    if (!active) {
      return
    }
    const signature = sourceStatusSignature(payload)
    const monitorsChanged = signature !== lastSignature
    lastSignature = signature
    scheduleSilentRefresh(monitorsChanged)
  }

  function scheduleSilentRefresh(includeMonitors: boolean) {
    pendingMonitorsRefresh = pendingMonitorsRefresh || includeMonitors
    if (silentRefreshHandle !== null) {
      return
    }
    silentRefreshHandle = window.setTimeout(() => {
      silentRefreshHandle = null
      void runSilentRefresh()
    }, silentRefreshDebounceMs)
  }

  async function runSilentRefresh() {
    if (!active || loading.value || restarting.value) {
      return
    }
    if (silentRefreshInFlight) {
      silentRefreshQueued = true
      return
    }
    const includeMonitors = pendingMonitorsRefresh
    pendingMonitorsRefresh = false
    silentRefreshInFlight = true
    try {
      const [statusResponse, monitorResponse] = await Promise.all([
        apiRequest<BilibiliSourceStatusResponse>('/api/bilibili/source/status'),
        includeMonitors
          ? apiRequest<ThirdPartyMonitorsResponse>(`/api/third-party/monitors?platform=${encodeURIComponent(platform.value)}`)
          : Promise.resolve(null),
      ])
      if (!active) {
        return
      }
      const resolvedMonitors = monitorResponse ? await resolveMonitorMedia(monitorResponse) : null
      if (!active) {
        return
      }
      bilibiliStatus.value = statusResponse
      if (resolvedMonitors) {
        monitors.value = resolvedMonitors
      }
      lastRefreshedAt.value = new Date().toISOString()
    } catch {
      // 静默刷新失败时保留上次成功数据，等待下一次事件信号
    } finally {
      silentRefreshInFlight = false
      if (active && silentRefreshQueued) {
        silentRefreshQueued = false
        scheduleSilentRefresh(pendingMonitorsRefresh)
      }
    }
  }

  async function resolveMonitorMedia(response: ThirdPartyMonitorsResponse) {
    const items = await Promise.all(response.items.map(async (item) => {
      const [avatarURL, coverURL, dynamicImages] = await Promise.all([
        downloadThirdPartyMedia(item.avatar_url),
        downloadThirdPartyMedia(item.live.cover_url),
        resolveDynamicImages(item.dynamic?.images ?? []),
      ])
      return {
        ...item,
        avatar_url: avatarURL || item.avatar_url,
        dynamic: item.dynamic
          ? {
              ...item.dynamic,
              images: dynamicImages,
            }
          : item.dynamic,
        live: {
          ...item.live,
          cover_url: coverURL || item.live.cover_url,
        },
      }
    }))
    return {
      ...response,
      items,
    }
  }

  async function resolveDynamicImages(images: NonNullable<ThirdPartyMonitorItem['dynamic']>['images']) {
    return Promise.all(images.map(async (image) => {
      const url = await downloadThirdPartyMedia(image.url)
      return {
        ...image,
        url: url || image.url,
      }
    }))
  }

  async function downloadThirdPartyMedia(url: string) {
    const normalizedURL = normalizeThirdPartyMediaURL(url)
    if (!normalizedURL) {
      return url
    }
    const cached = mediaObjectURLs.get(normalizedURL)
    if (cached) {
      return cached
    }
    try {
      const { blob } = await apiDownload(`/api/third-party/media?url=${encodeURIComponent(normalizedURL)}`)
      const objectURL = window.URL.createObjectURL(blob)
      mediaObjectURLs.set(normalizedURL, objectURL)
      return objectURL
    } catch {
      return url
    }
  }

  function disposeMedia() {
    for (const objectURL of mediaObjectURLs.values()) {
      window.URL.revokeObjectURL(objectURL)
    }
    mediaObjectURLs.clear()
    monitors.value = null
  }

  async function restartBilibiliSource() {
    restarting.value = true
    try {
      const response = await apiRequest<BilibiliSourceRestartResponse>('/api/bilibili/source/restart', {
        method: 'POST',
      })
      bilibiliStatus.value = response.status
      await fetchAll()
      return response
    } finally {
      restarting.value = false
    }
  }

  return {
    bilibiliStatus,
    error,
    items,
    lastRefreshedAt,
    loading,
    monitors,
    platform,
    restarting,
    activate,
    deactivate,
    fetchAll,
    handleSourceStatusEvent,
    restartBilibiliSource,
    disposeMedia,
  }
})

function sourceStatusSignature(payload: BilibiliSourceStatusEventPayload) {
  return [
    payload.status,
    payload.live_watched_rooms,
    payload.live_connected_rooms,
    payload.live_failed_rooms,
    payload.fallback_polling,
    payload.dynamic_enabled,
    payload.dynamic_watched_uids,
    payload.last_event_at ?? '',
    payload.last_error ? '1' : '0',
  ].join('|')
}

function signatureFromStatusResponse(response: BilibiliSourceStatusResponse) {
  return [
    response.status,
    response.live.watched_rooms,
    response.live.connected_rooms,
    response.live.failed_rooms,
    response.live.fallback_polling,
    response.dynamic.enabled,
    response.dynamic.watched_uids,
    newestEventTime(response.live.last_event_at, response.dynamic.last_event_at),
    response.live.last_error || response.dynamic.last_error ? '1' : '0',
  ].join('|')
}

function newestEventTime(...values: Array<string | null | undefined>) {
  let newest = ''
  let newestMs = Number.NEGATIVE_INFINITY
  for (const value of values) {
    if (!value) {
      continue
    }
    const ms = Date.parse(value)
    if (Number.isFinite(ms) && ms > newestMs) {
      newestMs = ms
      newest = value
    }
  }
  return newest
}

function normalizeThirdPartyMediaURL(value: string) {
  const text = value.trim()
  if (!text) {
    return ''
  }
  try {
    const parsed = new URL(text.startsWith('//') ? `https:${text}` : text)
    if ((parsed.protocol !== 'https:' && parsed.protocol !== 'http:') || !isBilibiliMediaHost(parsed.hostname)) {
      return ''
    }
    if (!parsed.pathname.startsWith('/bfs/') && !parsed.pathname.startsWith('/fs/')) {
      return ''
    }
    parsed.protocol = 'https:'
    parsed.search = ''
    parsed.hash = ''
    return parsed.toString()
  } catch {
    return ''
  }
}

function isBilibiliMediaHost(hostname: string) {
  const host = hostname.toLowerCase()
  return host === 'hdslb.com' || host.endsWith('.hdslb.com')
}
