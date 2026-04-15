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

  it('builds filtered log queries and only keeps the latest filtered response', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      items: [
        {
          log_id: 'log_warn_0001',
          timestamp: '2026-04-05T08:00:00Z',
          level: 'warn',
          source: 'adapter',
          plugin_id: 'weather',
          request_id: 'req_1',
          message: 'same message',
        },
        {
          log_id: 'log_warn_0001',
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
    store.items = [
      {
        log_id: 'log_old_0001',
        timestamp: '2026-04-05T07:59:00Z',
        level: 'info',
        source: 'runtime',
        message: 'stale item',
      },
    ]
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
    expect(store.items[0]?.log_id).toBe('log_warn_0001')
  })

  it('prepends appended logs and respects the current limit', () => {
    const store = useLogsStore()
    store.filters = { limit: 2 }
    store.items = [
      { log_id: 'log_info_0002', timestamp: '2026-04-05T08:00:01Z', level: 'info', source: 'system', message: 'older' },
      { log_id: 'log_info_0001', timestamp: '2026-04-05T08:00:00Z', level: 'info', source: 'system', message: 'oldest' },
    ]

    store.append({ log_id: 'log_error_0001', timestamp: '2026-04-05T08:00:02Z', level: 'error', source: 'system', message: 'latest' })

    expect(store.items.map((item) => item.message)).toEqual(['latest', 'older'])
  })

  it('marks the latest page for refresh when hidden logs arrive while the page is inactive', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      items: [
        {
          log_id: 'log_hidden_0002',
          timestamp: '2026-04-05T08:00:02Z',
          level: 'info',
          source: 'runtime',
          message: 'hidden latest',
        },
        {
          log_id: 'log_hidden_0001',
          timestamp: '2026-04-05T08:00:01Z',
          level: 'info',
          source: 'runtime',
          message: 'visible row',
        },
      ],
    }))
    vi.stubGlobal('fetch', fetchMock)

    const store = useLogsStore()
    store.items = [
      {
        log_id: 'log_hidden_0001',
        timestamp: '2026-04-05T08:00:01Z',
        level: 'info',
        source: 'runtime',
        message: 'visible row',
      },
    ]

    store.deactivate()
    store.append({
      log_id: 'log_hidden_0002',
      timestamp: '2026-04-05T08:00:02Z',
      level: 'info',
      source: 'runtime',
      message: 'hidden latest',
    })

    expect(store.items.map((item) => item.message)).toEqual(['visible row'])
    expect(store.needsLatestRefresh).toBe(true)

    store.activate()
    await store.goToLatestPage()

    expect(fetchMock).toHaveBeenCalledWith('/api/logs?limit=50', expect.any(Object))
    expect(store.items.map((item) => item.message)).toEqual(['hidden latest', 'visible row'])
    expect(store.needsLatestRefresh).toBe(false)
  })

  it('keeps visible live logs when the activation refresh returns a stale latest page', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      items: [
        {
          log_id: 'log_stale_older_0001',
          timestamp: '2026-04-05T08:00:00Z',
          level: 'info',
          source: 'runtime',
          request_id: 'req_logs_stale_1',
          message: 'older persisted row',
        },
      ],
    }))
    vi.stubGlobal('fetch', fetchMock)

    const store = useLogsStore()
    store.items = [
      {
        log_id: 'log_stale_older_0001',
        timestamp: '2026-04-05T08:00:00Z',
        level: 'info',
        source: 'runtime',
        request_id: 'req_logs_stale_1',
        message: 'older persisted row',
      },
    ]

    store.append({
      log_id: 'log_stale_visible_0001',
      timestamp: '2026-04-05T08:00:01Z',
      level: 'info',
      source: 'bridge',
      request_id: 'req_logs_stale_1',
      message: 'visible live row',
    })

    store.deactivate()
    store.append({
      log_id: 'log_stale_hidden_0001',
      timestamp: '2026-04-05T08:00:02Z',
      level: 'info',
      source: 'bridge',
      request_id: 'req_logs_stale_1',
      message: 'hidden live row',
    })

    store.activate()
    await store.restoreLatestPage()

    expect(fetchMock).toHaveBeenCalledWith('/api/logs?limit=50', expect.any(Object))
    expect(store.items.map((item) => item.message)).toEqual([
      'hidden live row',
      'visible live row',
      'older persisted row',
    ])
  })

  it('ignores live logs that do not match the active filters', () => {
    const store = useLogsStore()
    store.filters = {
      level: 'warn',
      source: 'adapter',
      pluginId: 'weather',
      limit: 5,
    }
    store.items = [
      {
        log_id: 'log_warn_0001',
        timestamp: '2026-04-05T08:00:00Z',
        level: 'warn',
        source: 'adapter',
        plugin_id: 'weather',
        message: 'kept',
      },
    ]

    store.append({
      log_id: 'log_error_0002',
      timestamp: '2026-04-05T08:00:03Z',
      level: 'error',
      source: 'runtime',
      plugin_id: 'other',
      message: 'should be ignored',
    })

    expect(store.items.map((item) => item.message)).toEqual(['kept'])
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
