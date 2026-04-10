import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { buildProtocolLogListPath, useProtocolLogsStore } from '@/stores/protocol-logs'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('protocol logs store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('builds protocol-scoped history queries and selects the latest detail', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({
        items: [
          {
            log_id: 'log_protocol_0001',
            timestamp: '2026-04-08T10:16:00Z',
            level: 'warn',
            protocol: 'onebot11',
            source: 'adapter.onebot11',
            message: 'ignored OneBot API response with unsupported echo',
            request_id: 'req_adapter_ignored_0001',
          },
        ],
      }))
      .mockResolvedValueOnce(jsonResponse({
        log_id: 'log_protocol_0001',
        timestamp: '2026-04-08T10:16:00Z',
        level: 'warn',
        protocol: 'onebot11',
        source: 'adapter.onebot11',
        message: 'ignored OneBot API response with unsupported echo',
        request_id: 'req_adapter_ignored_0001',
        details: {
          reason: 'api response echo must be a non-empty string',
        },
      }))
    vi.stubGlobal('fetch', fetchMock)

    const store = useProtocolLogsStore()
    store.activate()
    store.filters = {
      level: 'warn',
      source: 'adapter.onebot11',
      requestId: 'req_adapter_ignored_0001',
      limit: 200,
    }

    await store.fetchList()

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      '/api/logs?protocol=onebot11&limit=200&level=warn&source=adapter.onebot11&request_id=req_adapter_ignored_0001',
      expect.any(Object),
    )
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      '/api/logs/log_protocol_0001',
      expect.any(Object),
    )
    expect(store.selectedLogId).toBe('log_protocol_0001')
    expect(store.currentDetail?.details.reason).toBe('api response echo must be a non-empty string')
  })

  it('keeps matching live protocol logs even before the page activates', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      log_id: 'log_protocol_live_0001',
      timestamp: '2026-04-08T10:17:00Z',
      level: 'warn',
      protocol: 'onebot11',
      source: 'adapter.onebot11',
      message: 'ignored OneBot API response with unsupported echo',
      request_id: 'req_live_1',
      details: {
        echo_value_type: 'number',
      },
    }))
    vi.stubGlobal('fetch', fetchMock)

    const store = useProtocolLogsStore()
    store.filters = {
      level: 'warn',
      source: 'adapter.onebot11',
      requestId: 'req_live_1',
      limit: 200,
    }

    const accepted = await store.appendLive({
      log_id: 'log_protocol_live_0001',
      timestamp: '2026-04-08T10:17:00Z',
      level: 'warn',
      protocol: 'onebot11',
      source: 'adapter.onebot11',
      message: 'ignored OneBot API response with unsupported echo',
      request_id: 'req_live_1',
    })
    const rejected = await store.appendLive({
      log_id: 'log_runtime_0001',
      timestamp: '2026-04-08T10:17:01Z',
      level: 'warn',
      source: 'runtime',
      message: 'runtime only',
      request_id: 'req_live_1',
    })

    expect(accepted).toBe(true)
    expect(rejected).toBe(false)
    expect(store.items.map((item) => item.log_id)).toEqual(['log_protocol_live_0001'])
    expect(store.selectedLogId).toBeNull()
    expect(store.currentDetail).toBeNull()
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('loads live protocol log details while active and auto-following', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      log_id: 'log_protocol_live_0001',
      timestamp: '2026-04-08T10:17:00Z',
      level: 'warn',
      protocol: 'onebot11',
      source: 'adapter.onebot11',
      message: 'ignored OneBot API response with unsupported echo',
      request_id: 'req_live_1',
      details: {
        echo_value_type: 'number',
      },
    }))
    vi.stubGlobal('fetch', fetchMock)

    const store = useProtocolLogsStore()
    store.activate()

    const accepted = await store.appendLive({
      log_id: 'log_protocol_live_0001',
      timestamp: '2026-04-08T10:17:00Z',
      level: 'warn',
      protocol: 'onebot11',
      source: 'adapter.onebot11',
      message: 'ignored OneBot API response with unsupported echo',
      request_id: 'req_live_1',
    })

    expect(accepted).toBe(true)
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/logs/log_protocol_live_0001',
      expect.any(Object),
    )
    expect(store.selectedLogId).toBe('log_protocol_live_0001')
    expect(store.currentDetail?.details.echo_value_type).toBe('number')
  })

  it('keeps buffered live logs after the page refreshes history again', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({
        items: [
          {
            log_id: 'log_protocol_history_0001',
            timestamp: '2026-04-08T10:16:00Z',
            level: 'info',
            protocol: 'onebot11',
            source: 'adapter.onebot11',
            message: 'adapter connected',
            request_id: 'req_history_1',
          },
        ],
      }))
      .mockResolvedValueOnce(jsonResponse({
        log_id: 'log_protocol_live_0001',
        timestamp: '2026-04-08T10:17:00Z',
        level: 'warn',
        protocol: 'onebot11',
        source: 'adapter.onebot11',
        message: 'ignored OneBot API response with unsupported echo',
        request_id: 'req_live_1',
        details: {
          reason: 'api response echo must be a non-empty string',
        },
      }))
    vi.stubGlobal('fetch', fetchMock)

    const store = useProtocolLogsStore()
    store.activate()
    store.pauseAutoFollow()
    store.filters = {
      source: 'adapter.onebot11',
      limit: 200,
    }

    await store.appendLive({
      log_id: 'log_protocol_live_0001',
      timestamp: '2026-04-08T10:17:00Z',
      level: 'warn',
      protocol: 'onebot11',
      source: 'adapter.onebot11',
      message: 'ignored OneBot API response with unsupported echo',
      request_id: 'req_live_1',
    })

    await store.fetchList()

    expect(store.items.map((item) => item.log_id)).toEqual([
      'log_protocol_history_0001',
      'log_protocol_live_0001',
    ])
    expect(store.selectedLogId).toBe('log_protocol_live_0001')
    expect(store.currentDetail?.details.reason).toBe('api response echo must be a non-empty string')
  })

  it('exposes the fixed protocol path helper', () => {
    expect(buildProtocolLogListPath({
      source: 'adapter',
      limit: 50,
    })).toBe('/api/logs?protocol=onebot11&limit=50&source=adapter')
  })
})
