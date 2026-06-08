import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiDownload, apiRequest } from '@/lib/http'
import type {
  BilibiliSourceRestartResponse,
  BilibiliSourceStatusResponse,
  ThirdPartyMonitorItem,
  ThirdPartyMonitorsResponse,
  ThirdPartyPlatform,
} from '@/types/api'

export const useThirdPartyMonitoringStore = defineStore('third-party-monitoring', () => {
  const platform = ref<ThirdPartyPlatform>('bilibili')
  const monitors = ref<ThirdPartyMonitorsResponse | null>(null)
  const bilibiliStatus = ref<BilibiliSourceStatusResponse | null>(null)
  const loading = ref(false)
  const restarting = ref(false)
  const error = ref<string | null>(null)
  const mediaObjectURLs = new Map<string, string>()

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
    } catch (err) {
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      loading.value = false
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
    loading,
    monitors,
    platform,
    restarting,
    fetchAll,
    restartBilibiliSource,
    disposeMedia,
  }
})

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
