import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useLogsStore } from '@/stores/logs'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('logs store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('builds filtered log queries and merges duplicates', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      items: [
        {
          timestamp: '2026-04-05T08:00:00Z',
          level: 'warn',
          source: 'adapter',
          plugin_id: 'weather',
          request_id: 'req_1',
          message: 'same message',
        },
        {
          timestamp: '2026-04-05T08:00:00Z',
          level: 'warn',
          source: 'adapter',
          plugin_id: 'weather',
          request_id: 'req_1',
          message: 'same message',
        },
      ],
    }))
    vi.stubGlobal('fetch', fetchMock)

    const store = useLogsStore()
    store.filters = {
      level: 'warn',
      source: 'adapter',
      pluginId: 'weather',
      requestId: 'req_1',
      limit: 5,
    }

    await store.fetchList()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/logs?level=warn&source=adapter&plugin_id=weather&request_id=req_1&limit=5',
      expect.any(Object),
    )
    expect(store.items).toHaveLength(1)
  })

  it('prepends appended logs and respects the current limit', () => {
    const store = useLogsStore()
    store.filters = { limit: 2 }
    store.items = [
      { timestamp: '2026-04-05T08:00:01Z', level: 'info', source: 'system', message: 'older' },
      { timestamp: '2026-04-05T08:00:00Z', level: 'info', source: 'system', message: 'oldest' },
    ]

    store.append({ timestamp: '2026-04-05T08:00:02Z', level: 'error', source: 'system', message: 'latest' })

    expect(store.items.map((item) => item.message)).toEqual(['latest', 'older'])
  })

  it('stores a visible error when loading fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({
      error: {
        code: 'platform.unknown',
        message: '读取日志失败',
        request_id: 'req_logs_1',
      },
    }, 500)))

    const store = useLogsStore()
    await expect(store.fetchList()).rejects.toMatchObject({ message: '读取日志失败' })
    expect(store.error).toBe('读取日志失败')
  })
})
