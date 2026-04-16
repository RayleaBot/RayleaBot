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

  it('loads current session logs in ascending order', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      items: [
        {
          log_id: 'log_current_0002',
          timestamp: '2026-04-05T08:00:02Z',
          level: 'warn',
          source: 'adapter',
          message: 'later row',
        },
        {
          log_id: 'log_current_0001',
          timestamp: '2026-04-05T08:00:01Z',
          level: 'info',
          source: 'runtime',
          message: 'earlier row',
        },
      ],
      page: {
        limit: 100,
        has_older: true,
        older_cursor: 'cursor-older-1',
      },
    }))
    vi.stubGlobal('fetch', fetchMock)

    const store = useLogsStore()
    await store.ensureLoaded()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/logs?scope=current_session&limit=100',
      expect.any(Object),
    )
    expect(store.items.map((item) => item.log_id)).toEqual([
      'log_current_0001',
      'log_current_0002',
    ])
    expect(store.initialized).toBe(true)
    expect(store.hasOlder).toBe(true)
  })

  it('replaces items and sends current-session filters when applying filters', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      items: [
        {
          log_id: 'log_warn_0001',
          timestamp: '2026-04-05T08:00:00Z',
          level: 'warn',
          protocol: 'onebot11',
          source: 'adapter',
          plugin_id: 'weather',
          request_id: 'req_1',
          message: 'filtered row',
        },
      ],
      page: {
        limit: 100,
        has_older: false,
        older_cursor: null,
      },
    }))
    vi.stubGlobal('fetch', fetchMock)

    const store = useLogsStore()
    store.items = [
      {
        log_id: 'log_stale_0001',
        timestamp: '2026-04-05T07:59:00Z',
        level: 'info',
        source: 'runtime',
        message: 'stale row',
      },
    ]
    store.pendingNewCount = 2
    store.filters = {
      level: 'warn',
      source: 'adapter',
      protocol: 'onebot11',
      pluginId: 'weather',
      requestId: 'req_1',
    }

    await store.applyFilters()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/logs?scope=current_session&limit=100&level=warn&source=adapter&protocol=onebot11&plugin_id=weather&request_id=req_1',
      expect.any(Object),
    )
    expect(store.items.map((item) => item.log_id)).toEqual(['log_warn_0001'])
    expect(store.pendingNewCount).toBe(0)
  })

  it('loads older current-session rows at the top without breaking ascending order', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({
        items: [
          {
            log_id: 'log_current_0002',
            timestamp: '2026-04-05T08:00:02Z',
            level: 'warn',
            source: 'adapter',
            message: 'later row',
          },
          {
            log_id: 'log_current_0001',
            timestamp: '2026-04-05T08:00:01Z',
            level: 'info',
            source: 'runtime',
            message: 'earlier row',
          },
        ],
        page: {
          limit: 100,
          has_older: true,
          older_cursor: 'cursor-older-1',
        },
      }))
      .mockResolvedValueOnce(jsonResponse({
        items: [
          {
            log_id: 'log_current_0000',
            timestamp: '2026-04-05T07:59:59Z',
            level: 'info',
            source: 'runtime',
            message: 'oldest row',
          },
        ],
        page: {
          limit: 100,
          has_older: false,
          older_cursor: null,
        },
      }))
    vi.stubGlobal('fetch', fetchMock)

    const store = useLogsStore()
    await store.ensureLoaded()
    await store.loadOlder()

    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      '/api/logs?scope=current_session&limit=100&cursor=cursor-older-1&direction=older',
      expect.any(Object),
    )
    expect(store.items.map((item) => item.log_id)).toEqual([
      'log_current_0000',
      'log_current_0001',
      'log_current_0002',
    ])
    expect(store.hasOlder).toBe(false)
  })

  it('tracks pending live rows away from the bottom and ignores mismatched filters', () => {
    const store = useLogsStore()
    store.filters = {
      level: 'warn',
      source: 'adapter',
    }
    store.setViewportActive(false)
    store.setViewportAtBottom(false)

    const accepted = store.append({
      log_id: 'log_warn_0001',
      timestamp: '2026-04-05T08:00:01Z',
      level: 'warn',
      source: 'adapter',
      message: 'matching row',
    })
    const rejected = store.append({
      log_id: 'log_info_0001',
      timestamp: '2026-04-05T08:00:02Z',
      level: 'info',
      source: 'runtime',
      message: 'ignored row',
    })

    expect(accepted).toBe(true)
    expect(rejected).toBe(false)
    expect(store.items.map((item) => item.log_id)).toEqual(['log_warn_0001'])
    expect(store.pendingNewCount).toBe(1)

    store.setViewportActive(true)
    store.setViewportAtBottom(true)
    store.append({
      log_id: 'log_warn_0002',
      timestamp: '2026-04-05T08:00:03Z',
      level: 'warn',
      source: 'adapter',
      message: 'latest row',
    })

    expect(store.items.map((item) => item.log_id)).toEqual([
      'log_warn_0001',
      'log_warn_0002',
    ])
    expect(store.pendingNewCount).toBe(0)
  })

  it('keeps already seen live rows when refreshing latest data', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      items: [
        {
          log_id: 'log_persisted_0001',
          timestamp: '2026-04-05T08:00:00Z',
          level: 'info',
          source: 'runtime',
          message: 'persisted row',
        },
      ],
      page: {
        limit: 100,
        has_older: false,
        older_cursor: null,
      },
    }))
    vi.stubGlobal('fetch', fetchMock)

    const store = useLogsStore()
    store.items = [
      {
        log_id: 'log_persisted_0001',
        timestamp: '2026-04-05T08:00:00Z',
        level: 'info',
        source: 'runtime',
        message: 'persisted row',
      },
    ]
    store.append({
      log_id: 'log_live_0001',
      timestamp: '2026-04-05T08:00:01Z',
      level: 'info',
      source: 'bridge',
      message: 'live row',
    })

    await store.refreshLatest()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/logs?scope=current_session&limit=100',
      expect.any(Object),
    )
    expect(store.items.map((item) => item.log_id)).toEqual([
      'log_persisted_0001',
      'log_live_0001',
    ])
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
    await expect(store.ensureLoaded()).rejects.toMatchObject({ message: '读取日志失败' })
    expect(store.error).toBe('读取日志失败')
  })
})
