import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useThirdPartyMonitoringStore } from '@/stores/third-party-monitoring'
import type {
  BilibiliSourceStatusEventPayload,
  BilibiliSourceStatusResponse,
  ThirdPartyMonitorsResponse,
} from '@/types/api'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function imageResponse() {
  return new Response(new Blob(['avatar'], { type: 'image/png' }), {
    status: 200,
    headers: { 'Content-Type': 'image/png' },
  })
}

describe('third-party monitoring store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('downloads protocol-relative Bilibili monitor media through the controlled endpoint', async () => {
    const originalCreateObjectURL = window.URL.createObjectURL
    const originalRevokeObjectURL = window.URL.revokeObjectURL
    const createObjectURL = vi.fn()
      .mockReturnValueOnce('blob:monitor-avatar')
      .mockReturnValueOnce('blob:monitor-dynamic')
    const revokeObjectURL = vi.fn()
    Object.defineProperty(window.URL, 'createObjectURL', {
      configurable: true,
      writable: true,
      value: createObjectURL,
    })
    Object.defineProperty(window.URL, 'revokeObjectURL', {
      configurable: true,
      writable: true,
      value: revokeObjectURL,
    })

    vi.stubGlobal('fetch', vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const path = input.toString()
      if (path.startsWith('/api/third-party/monitors')) {
        return Promise.resolve(jsonResponse({
          platform: 'bilibili',
          items: [
            {
              uid: '123456',
              username: '测试 UP',
              avatar_url: '//i0.hdslb.com/bfs/face/up.jpg@96w_96h_1c.png?from=monitor',
              profile_url: 'https://space.bilibili.com/123456/',
              services: ['live'],
              dynamic: {
                last_id: '90001',
                service: 'video',
                title: '新视频标题',
                summary: '视频简介',
                url: 'https://www.bilibili.com/video/BV1RayleaBot',
                images: [
                  {
                    url: 'http://i0.hdslb.com/bfs/archive/cover.jpg?from=monitor',
                    width: 1920,
                    height: 1080,
                  },
                ],
                published_at: null,
                observed_at: '2026-06-08T08:11:05Z',
              },
              live: {
                room_id: '10001',
                room_name: '直播间标题',
                room_url: 'https://live.bilibili.com/10001',
                cover_url: '',
                is_live: true,
                live_started_at: null,
                live_ended_at: null,
                connection_state: 'connected',
                last_error: '',
                updated_at: null,
              },
              updated_at: '2026-06-08T08:11:05Z',
            },
          ],
          updated_at: '2026-06-08T08:11:05Z',
        }))
      }
      if (path === '/api/bilibili/source/status') {
        return Promise.resolve(jsonResponse({
          status: 'connected',
          summary: 'Bilibili 事件源运行中',
          live: {
            watched_rooms: 1,
            connected_rooms: 1,
            failed_rooms: 0,
            fallback_polling: true,
            last_event_at: null,
            last_error: '',
          },
          dynamic: {
            enabled: true,
            interval_seconds: 10,
            watched_uids: 1,
            auto_follow: true,
            last_poll_at: null,
            last_event_at: null,
            last_error: '',
          },
          diagnosis: {
            level: 'normal',
            headline: 'Bilibili 事件源运行中',
            description: '直播和动态检查正在正常运行。',
            causes: [
              {
                scope: 'source',
                code: 'healthy',
                title: '检查正常',
                detail: '直播和动态检查正在按当前配置运行。',
                last_error: '',
                retry_at: null,
              },
            ],
            impacts: ['直播状态正常检查。', '动态接收不受影响。', 'CK 有效，无需重新登录。'],
            actions: [
              { kind: 'refresh', label: '刷新状态', target: null, primary: true },
            ],
            updated_at: '2026-06-08T08:11:05Z',
          },
          accounts: [],
        }))
      }
      if (path === '/api/third-party/media?url=https%3A%2F%2Fi0.hdslb.com%2Fbfs%2Fface%2Fup.jpg%4096w_96h_1c.png') {
        return Promise.resolve(imageResponse())
      }
      if (path === '/api/third-party/media?url=https%3A%2F%2Fi0.hdslb.com%2Fbfs%2Farchive%2Fcover.jpg') {
        return Promise.resolve(imageResponse())
      }
      return Promise.resolve(jsonResponse({ error: { message: 'unexpected request' } }, 500))
    }))

    const store = useThirdPartyMonitoringStore()

    try {
      await store.fetchAll()

      expect(store.items[0]?.avatar_url).toBe('blob:monitor-avatar')
      expect(store.items[0]?.dynamic?.images[0]?.url).toBe('blob:monitor-dynamic')
      expect(fetch).toHaveBeenCalledWith(
        '/api/third-party/media?url=https%3A%2F%2Fi0.hdslb.com%2Fbfs%2Fface%2Fup.jpg%4096w_96h_1c.png',
        expect.any(Object),
      )
      expect(fetch).toHaveBeenCalledWith(
        '/api/third-party/media?url=https%3A%2F%2Fi0.hdslb.com%2Fbfs%2Farchive%2Fcover.jpg',
        expect.any(Object),
      )

      store.disposeMedia()

      expect(revokeObjectURL).toHaveBeenCalledWith('blob:monitor-avatar')
      expect(revokeObjectURL).toHaveBeenCalledWith('blob:monitor-dynamic')
    } finally {
      Object.defineProperty(window.URL, 'createObjectURL', {
        configurable: true,
        writable: true,
        value: originalCreateObjectURL,
      })
      Object.defineProperty(window.URL, 'revokeObjectURL', {
        configurable: true,
        writable: true,
        value: originalRevokeObjectURL,
      })
    }
  })
})

function silentMonitorsBody(): ThirdPartyMonitorsResponse {
  return {
    platform: 'bilibili',
    items: [
      {
        uid: '123456',
        username: '测试 UP',
        avatar_url: '',
        profile_url: 'https://space.bilibili.com/123456/',
        services: ['live'],
        dynamic: null,
        live: {
          room_id: '10001',
          room_name: '直播间标题',
          room_url: 'https://live.bilibili.com/10001',
          cover_url: '',
          is_live: true,
          live_started_at: null,
          live_ended_at: null,
          connection_state: 'connected',
          last_error: '',
          updated_at: null,
        },
        updated_at: '2026-06-08T08:11:05Z',
      },
    ],
    updated_at: '2026-06-08T08:11:05Z',
  }
}

function silentStatusBody(): BilibiliSourceStatusResponse {
  return {
    status: 'connected',
    summary: 'Bilibili 事件源运行中',
    live: {
      watched_rooms: 1,
      connected_rooms: 1,
      failed_rooms: 0,
      fallback_polling: false,
      last_event_at: null,
      last_error: '',
    },
    dynamic: {
      enabled: true,
      interval_seconds: 10,
      watched_uids: 1,
      auto_follow: true,
      last_poll_at: null,
      last_event_at: null,
      last_error: '',
    },
    diagnosis: {
      level: 'normal',
      headline: 'Bilibili 事件源运行中',
      description: '直播和动态检查正在正常运行。',
      causes: [],
      impacts: [],
      actions: [],
      updated_at: '2026-06-08T08:11:05Z',
    },
    accounts: [],
  }
}

function sourceEventPayload(overrides: Partial<BilibiliSourceStatusEventPayload> = {}): BilibiliSourceStatusEventPayload {
  return {
    source: 'bilibili',
    status: 'connected',
    summary: 'Bilibili 事件源运行中',
    live_watched_rooms: 1,
    live_connected_rooms: 1,
    live_failed_rooms: 0,
    fallback_polling: false,
    dynamic_enabled: true,
    dynamic_watched_uids: 1,
    last_event_at: null,
    last_error: '',
    diagnosis: {
      level: 'normal',
      headline: 'Bilibili 事件源运行中',
      description: '直播和动态检查正在正常运行。',
      causes: [],
      impacts: [],
      actions: [],
      updated_at: '2026-06-08T08:11:05Z',
    },
    ...overrides,
  }
}

function installMonitoringFetch() {
  const calls = { monitors: 0, status: 0 }
  vi.stubGlobal('fetch', vi.fn().mockImplementation((input: RequestInfo | URL) => {
    const path = input.toString()
    if (path.startsWith('/api/third-party/monitors')) {
      calls.monitors += 1
      return Promise.resolve(jsonResponse(silentMonitorsBody()))
    }
    if (path === '/api/bilibili/source/status') {
      calls.status += 1
      return Promise.resolve(jsonResponse(silentStatusBody()))
    }
    return Promise.resolve(jsonResponse({ error: { message: 'unexpected request' } }, 500))
  }))
  return calls
}

describe('third-party monitoring store source status events', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('ignores source status events while the page is inactive', async () => {
    const calls = installMonitoringFetch()
    const store = useThirdPartyMonitoringStore()

    store.handleSourceStatusEvent(sourceEventPayload())
    await vi.advanceTimersByTimeAsync(300)

    expect(calls.status).toBe(0)
    expect(calls.monitors).toBe(0)
  })

  it('refreshes status and monitors silently on the first event after activation', async () => {
    const calls = installMonitoringFetch()
    const store = useThirdPartyMonitoringStore()

    store.activate()
    store.handleSourceStatusEvent(sourceEventPayload())
    expect(store.loading).toBe(false)

    await vi.advanceTimersByTimeAsync(120)

    expect(calls.status).toBe(1)
    expect(calls.monitors).toBe(1)
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
    expect(store.bilibiliStatus?.summary).toBe('Bilibili 事件源运行中')
    expect(store.items).toHaveLength(1)
  })

  it('skips the monitors refetch when the event signature is unchanged', async () => {
    const calls = installMonitoringFetch()
    const store = useThirdPartyMonitoringStore()

    store.activate()
    store.handleSourceStatusEvent(sourceEventPayload())
    await vi.advanceTimersByTimeAsync(120)
    expect(calls.monitors).toBe(1)

    store.handleSourceStatusEvent(sourceEventPayload())
    await vi.advanceTimersByTimeAsync(120)

    expect(calls.status).toBe(2)
    expect(calls.monitors).toBe(1)
  })

  it('refetches monitors when the event signature changes', async () => {
    const calls = installMonitoringFetch()
    const store = useThirdPartyMonitoringStore()

    store.activate()
    store.handleSourceStatusEvent(sourceEventPayload())
    await vi.advanceTimersByTimeAsync(120)
    expect(calls.monitors).toBe(1)

    store.handleSourceStatusEvent(sourceEventPayload({ last_event_at: '2026-06-08T09:00:00Z' }))
    await vi.advanceTimersByTimeAsync(120)

    expect(calls.monitors).toBe(2)
  })

  it('does not treat cooldown countdown text changes as monitor changes', async () => {
    const calls = installMonitoringFetch()
    const store = useThirdPartyMonitoringStore()

    store.activate()
    store.handleSourceStatusEvent(sourceEventPayload({ last_error: 'Bilibili 动态检查因平台风控暂停，剩余 5m0s' }))
    await vi.advanceTimersByTimeAsync(120)
    expect(calls.monitors).toBe(1)

    store.handleSourceStatusEvent(sourceEventPayload({ last_error: 'Bilibili 动态检查因平台风控暂停，剩余 4m45s' }))
    await vi.advanceTimersByTimeAsync(120)

    expect(calls.status).toBe(2)
    expect(calls.monitors).toBe(1)
  })

  it('keeps the last data and stays quiet when a silent refresh fails', async () => {
    installMonitoringFetch()
    const store = useThirdPartyMonitoringStore()

    store.activate()
    store.handleSourceStatusEvent(sourceEventPayload())
    await vi.advanceTimersByTimeAsync(120)
    expect(store.items).toHaveLength(1)

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ error: { message: 'boom' } }, 500)))
    store.handleSourceStatusEvent(sourceEventPayload({ last_event_at: '2026-06-08T09:00:00Z' }))
    await vi.advanceTimersByTimeAsync(120)

    expect(store.items).toHaveLength(1)
    expect(store.bilibiliStatus?.summary).toBe('Bilibili 事件源运行中')
    expect(store.error).toBeNull()
    expect(store.loading).toBe(false)
  })

  it('seeds the signature from fetchAll so a matching frame skips the monitors refetch', async () => {
    const calls = installMonitoringFetch()
    const store = useThirdPartyMonitoringStore()

    store.activate()
    await store.fetchAll()
    expect(calls.monitors).toBe(1)
    expect(calls.status).toBe(1)

    store.handleSourceStatusEvent(sourceEventPayload())
    await vi.advanceTimersByTimeAsync(120)

    expect(calls.monitors).toBe(1)
    expect(calls.status).toBe(2)
  })

  it('updates lastRefreshedAt on fetchAll and silent refresh', async () => {
    const calls = installMonitoringFetch()
    const store = useThirdPartyMonitoringStore()

    expect(store.lastRefreshedAt).toBeNull()

    store.activate()
    await store.fetchAll()
    expect(store.lastRefreshedAt).not.toBeNull()
    expect(store.lastRefreshedAt).toMatch(/^\d{4}-\d{2}-\d{2}T/)

    const beforeSilent = store.lastRefreshedAt
    vi.advanceTimersByTime(10)
    store.handleSourceStatusEvent(sourceEventPayload({ last_event_at: '2026-06-08T09:00:00Z' }))
    await vi.advanceTimersByTimeAsync(120)

    expect(store.lastRefreshedAt).not.toBeNull()
    expect(store.lastRefreshedAt).not.toBe(beforeSilent)
    expect(calls.monitors).toBe(2)
  })

  it('clears lastRefreshedAt on deactivate', async () => {
    installMonitoringFetch()
    const store = useThirdPartyMonitoringStore()

    store.activate()
    await store.fetchAll()
    expect(store.lastRefreshedAt).not.toBeNull()

    store.deactivate()
    expect(store.lastRefreshedAt).toBeNull()
  })

  it('drops events while an explicit fetch is in flight', async () => {
    const calls = { monitors: 0, status: 0 }
    vi.stubGlobal('fetch', vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const path = input.toString()
      if (path.startsWith('/api/third-party/monitors')) {
        calls.monitors += 1
      } else if (path === '/api/bilibili/source/status') {
        calls.status += 1
      }
      return new Promise<never>(() => {})
    }))
    const store = useThirdPartyMonitoringStore()

    store.activate()
    void store.fetchAll().catch(() => {})
    expect(store.loading).toBe(true)

    store.handleSourceStatusEvent(sourceEventPayload())
    await vi.advanceTimersByTimeAsync(300)

    expect(calls.status).toBe(1)
    expect(calls.monitors).toBe(1)
  })

  it('runs one queued silent refresh when an event arrives during an in-flight silent refresh', async () => {
    const calls = { monitors: 0, status: 0 }
    let resolveFirstStatus: ((value: Response) => void) | null = null
    vi.stubGlobal('fetch', vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const path = input.toString()
      if (path.startsWith('/api/third-party/monitors')) {
        calls.monitors += 1
        return Promise.resolve(jsonResponse(silentMonitorsBody()))
      }
      if (path === '/api/bilibili/source/status') {
        calls.status += 1
        if (calls.status === 1) {
          return new Promise<Response>((resolve) => {
            resolveFirstStatus = resolve
          })
        }
        return Promise.resolve(jsonResponse(silentStatusBody()))
      }
      return Promise.resolve(jsonResponse({ error: { message: 'unexpected request' } }, 500))
    }))
    const store = useThirdPartyMonitoringStore()

    store.activate()
    store.handleSourceStatusEvent(sourceEventPayload())
    await vi.advanceTimersByTimeAsync(120)
    expect(calls.status).toBe(1)

    store.handleSourceStatusEvent(sourceEventPayload({ last_event_at: '2026-06-08T09:00:00Z' }))
    await vi.advanceTimersByTimeAsync(120)
    expect(calls.status).toBe(1)

    resolveFirstStatus?.(jsonResponse(silentStatusBody()))
    await vi.runOnlyPendingTimersAsync()
    await vi.advanceTimersByTimeAsync(120)

    expect(calls.status).toBe(2)
    expect(calls.monitors).toBe(2)
  })

  it('clears the pending refresh and media on deactivate', async () => {
    const calls = installMonitoringFetch()
    const store = useThirdPartyMonitoringStore()

    store.activate()
    store.handleSourceStatusEvent(sourceEventPayload())
    store.deactivate()
    await vi.advanceTimersByTimeAsync(300)

    expect(calls.status).toBe(0)
    expect(calls.monitors).toBe(0)
    expect(store.monitors).toBeNull()
  })
})
