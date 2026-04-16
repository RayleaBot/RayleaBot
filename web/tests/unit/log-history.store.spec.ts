import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { localDateTimeToUtc, toLocalDateTimeInput, useLogHistoryStore } from '@/stores/log-history'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('log history store', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-04-17T10:30:00Z'))
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('anchors to the most recent day and queries history in UTC', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      items: [
        {
          log_id: 'log_history_0001',
          timestamp: '2026-04-17T09:30:00Z',
          level: 'info',
          source: 'runtime',
          message: 'history row',
        },
      ],
      page: {
        limit: 100,
        has_older: false,
        older_cursor: null,
      },
    }))
    vi.stubGlobal('fetch', fetchMock)

    const anchor = new Date('2026-04-17T10:30:00Z')
    const start = new Date(anchor.getTime() - 24 * 60 * 60 * 1000)
    const store = useLogHistoryStore()

    await store.refreshAnchor()

    expect(store.anchorAt).toBe(anchor.toISOString())
    expect(store.timeRangeInput).toEqual({
      startLocal: toLocalDateTimeInput(start),
      endLocal: toLocalDateTimeInput(anchor),
    })
    expect(fetchMock).toHaveBeenCalledWith(
      `/api/logs?scope=history&limit=100&start_at=${encodeURIComponent(start.toISOString().replace('.000Z', 'Z'))}&end_at=${encodeURIComponent(anchor.toISOString().replace('.000Z', 'Z'))}`,
      expect.any(Object),
    )
    expect(store.items.map((item) => item.log_id)).toEqual(['log_history_0001'])
  })

  it('uses the selected local time range for filtering and loading older history rows', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({
        items: [
          {
            log_id: 'log_history_0002',
            timestamp: '2026-04-16T10:00:00Z',
            level: 'warn',
            protocol: 'onebot11',
            source: 'adapter',
            plugin_id: 'weather',
            request_id: 'req_1',
            message: 'newer history row',
          },
        ],
        page: {
          limit: 100,
          has_older: true,
          older_cursor: 'history-cursor-1',
        },
      }))
      .mockResolvedValueOnce(jsonResponse({
        items: [
          {
            log_id: 'log_history_0001',
            timestamp: '2026-04-16T09:00:00Z',
            level: 'warn',
            protocol: 'onebot11',
            source: 'adapter',
            plugin_id: 'weather',
            request_id: 'req_1',
            message: 'older history row',
          },
        ],
        page: {
          limit: 100,
          has_older: false,
          older_cursor: null,
        },
      }))
    vi.stubGlobal('fetch', fetchMock)

    const store = useLogHistoryStore()
    store.filters = {
      level: 'warn',
      source: 'adapter',
      protocol: 'onebot11',
      pluginId: 'weather',
      requestId: 'req_1',
    }
    store.timeRangeInput = {
      startLocal: '2026-04-16T08:00',
      endLocal: '2026-04-16T10:30',
    }

    await store.applyFilters()
    await store.loadOlder()

    const startAt = encodeURIComponent(localDateTimeToUtc('2026-04-16T08:00'))
    const endAt = encodeURIComponent(localDateTimeToUtc('2026-04-16T10:30'))
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      `/api/logs?scope=history&limit=100&level=warn&source=adapter&protocol=onebot11&plugin_id=weather&request_id=req_1&start_at=${startAt}&end_at=${endAt}`,
      expect.any(Object),
    )
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      `/api/logs?scope=history&limit=100&level=warn&source=adapter&protocol=onebot11&plugin_id=weather&request_id=req_1&start_at=${startAt}&end_at=${endAt}&cursor=history-cursor-1&direction=older`,
      expect.any(Object),
    )
    expect(store.customTimeRange).toBe(true)
    expect(store.items.map((item) => item.log_id)).toEqual([
      'log_history_0001',
      'log_history_0002',
    ])
    expect(store.hasOlder).toBe(false)
  })

  it('resets the default window after returning to the recent-day shortcut', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      items: [],
      page: {
        limit: 100,
        has_older: false,
        older_cursor: null,
      },
    }))
    vi.stubGlobal('fetch', fetchMock)

    const store = useLogHistoryStore()
    store.customTimeRange = true
    store.timeRangeInput = {
      startLocal: '2026-04-01T00:00',
      endLocal: '2026-04-02T00:00',
    }

    store.resetTimeRangeToDefault()
    await store.refreshAnchor()

    expect(store.customTimeRange).toBe(false)
    expect(store.timeRangeInput.startLocal).not.toBe('2026-04-01T00:00')
    expect(store.timeRangeInput.endLocal).not.toBe('2026-04-02T00:00')
  })
})
