import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useThirdPartyMonitoringStore } from '@/stores/third-party-monitoring'

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
