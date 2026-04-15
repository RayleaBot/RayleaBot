import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { buildProtocolLogListPath, useProtocolLogsStore } from '@/stores/protocol-logs'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function readSearchParams(input: string) {
  return new URL(input, 'http://localhost').searchParams
}

function createDeferredResponse() {
  let resolve!: (value: Response) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<Response>((promiseResolve, promiseReject) => {
    resolve = promiseResolve
    reject = promiseReject
  })
  return {
    promise,
    resolve,
    reject,
  }
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
        page: {
          limit: 200,
          has_older: false,
          has_newer: false,
          older_cursor: null,
          newer_cursor: null,
        },
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

    const listRequest = fetchMock.mock.calls[0]?.[0]
    expect(typeof listRequest).toBe('string')
    const params = readSearchParams(listRequest as string)
    expect(params.get('protocol')).toBe('onebot11')
    expect(params.get('limit')).toBe('200')
    expect(params.get('level')).toBe('warn')
    expect(params.get('source')).toBe('adapter.onebot11')
    expect(params.get('request_id')).toBe('req_adapter_ignored_0001')
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

  it('loads live protocol log details while active on the latest page', async () => {
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

  it('keeps history pages stable and counts newer live logs separately', async () => {
    vi.stubGlobal('fetch', vi.fn())

    const store = useProtocolLogsStore()
    store.activate()
    store.filters = {
      source: 'adapter.onebot11',
      limit: 200,
    }
    store.items = [
      {
        log_id: 'log_protocol_history_0001',
        timestamp: '2026-04-08T10:16:00Z',
        level: 'info',
        protocol: 'onebot11',
        source: 'adapter.onebot11',
        message: 'adapter connected',
        request_id: 'req_history_1',
      },
    ]
    store.isLatestPage = false
    store.selectedLogId = 'log_protocol_history_0001'

    await store.appendLive({
      log_id: 'log_protocol_live_0001',
      timestamp: '2026-04-08T10:17:00Z',
      level: 'warn',
      protocol: 'onebot11',
      source: 'adapter.onebot11',
      message: 'ignored OneBot API response with unsupported echo',
      request_id: 'req_live_1',
    })

    expect(store.items.map((item) => item.log_id)).toEqual(['log_protocol_history_0001'])
    expect(store.pendingNewCount).toBe(1)
    expect(store.selectedLogId).toBe('log_protocol_history_0001')
  })

  it('marks the latest protocol page for refresh when hidden logs arrive while inactive', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({
        items: [
          {
            log_id: 'log_protocol_hidden_0002',
            timestamp: '2026-04-08T10:18:00Z',
            level: 'info',
            protocol: 'onebot11',
            source: 'bridge',
            request_id: 'req_protocol_hidden_1',
            message: 'hidden protocol latest',
          },
          {
            log_id: 'log_protocol_hidden_0001',
            timestamp: '2026-04-08T10:17:00Z',
            level: 'info',
            protocol: 'onebot11',
            source: 'bridge',
            request_id: 'req_protocol_hidden_1',
            message: 'visible protocol row',
          },
        ],
        page: {
          limit: 200,
          has_older: false,
          has_newer: false,
          older_cursor: null,
          newer_cursor: null,
        },
      }))
      .mockResolvedValueOnce(jsonResponse({
        log_id: 'log_protocol_hidden_0002',
        timestamp: '2026-04-08T10:18:00Z',
        level: 'info',
        protocol: 'onebot11',
        source: 'bridge',
        request_id: 'req_protocol_hidden_1',
        message: 'hidden protocol latest',
        details: {
          direction: 'inbound',
          plain_text: 'hidden protocol latest',
        },
      }))
    vi.stubGlobal('fetch', fetchMock)

    const store = useProtocolLogsStore()
    store.items = [
      {
        log_id: 'log_protocol_hidden_0001',
        timestamp: '2026-04-08T10:17:00Z',
        level: 'info',
        protocol: 'onebot11',
        source: 'bridge',
        request_id: 'req_protocol_hidden_1',
        message: 'visible protocol row',
      },
    ]

    store.deactivate()
    await store.appendLive({
      log_id: 'log_protocol_hidden_0002',
      timestamp: '2026-04-08T10:18:00Z',
      level: 'info',
      protocol: 'onebot11',
      source: 'bridge',
      request_id: 'req_protocol_hidden_1',
      message: 'hidden protocol latest',
    })

    expect(store.items.map((item) => item.message)).toEqual(['visible protocol row'])
    expect(store.needsLatestRefresh).toBe(true)

    store.activate()
    await store.goToLatestPage()

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      '/api/logs?protocol=onebot11&limit=200',
      expect.any(Object),
    )
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      '/api/logs/log_protocol_hidden_0002',
      expect.any(Object),
    )
    expect(store.items.map((item) => item.message)).toEqual(['hidden protocol latest', 'visible protocol row'])
    expect(store.selectedLogId).toBe('log_protocol_hidden_0002')
    expect(store.needsLatestRefresh).toBe(false)
  })

  it('keeps visible live protocol logs when the activation refresh returns a stale latest page', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({
        log_id: 'log_protocol_visible_0001',
        timestamp: '2026-04-08T10:17:00Z',
        level: 'info',
        protocol: 'onebot11',
        source: 'bridge',
        request_id: 'req_protocol_stale_1',
        message: 'visible protocol row',
        details: {
          direction: 'inbound',
          plain_text: 'visible protocol row',
        },
      }))
      .mockResolvedValueOnce(jsonResponse({
        items: [
          {
            log_id: 'log_protocol_persisted_0001',
            timestamp: '2026-04-08T10:16:00Z',
            level: 'info',
            protocol: 'onebot11',
            source: 'adapter.onebot11',
            request_id: 'req_protocol_stale_1',
            message: 'persisted protocol row',
          },
        ],
        page: {
          limit: 200,
          has_older: false,
          has_newer: false,
          older_cursor: null,
          newer_cursor: null,
        },
      }))
      .mockResolvedValueOnce(jsonResponse({
        log_id: 'log_protocol_hidden_0003',
        timestamp: '2026-04-08T10:18:00Z',
        level: 'info',
        protocol: 'onebot11',
        source: 'bridge',
        request_id: 'req_protocol_stale_1',
        message: 'hidden protocol row',
        details: {
          direction: 'inbound',
          plain_text: 'hidden protocol row',
        },
      }))
    vi.stubGlobal('fetch', fetchMock)

    const store = useProtocolLogsStore()
    store.activate()
    store.items = [
      {
        log_id: 'log_protocol_persisted_0001',
        timestamp: '2026-04-08T10:16:00Z',
        level: 'info',
        protocol: 'onebot11',
        source: 'adapter.onebot11',
        request_id: 'req_protocol_stale_1',
        message: 'persisted protocol row',
      },
    ]

    await store.appendLive({
      log_id: 'log_protocol_visible_0001',
      timestamp: '2026-04-08T10:17:00Z',
      level: 'info',
      protocol: 'onebot11',
      source: 'bridge',
      request_id: 'req_protocol_stale_1',
      message: 'visible protocol row',
    })

    store.deactivate()
    await store.appendLive({
      log_id: 'log_protocol_hidden_0003',
      timestamp: '2026-04-08T10:18:00Z',
      level: 'info',
      protocol: 'onebot11',
      source: 'bridge',
      request_id: 'req_protocol_stale_1',
      message: 'hidden protocol row',
    })

    store.activate()
    await store.restoreLatestPage()

    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      '/api/logs?protocol=onebot11&limit=200',
      expect.any(Object),
    )
    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      '/api/logs/log_protocol_hidden_0003',
      expect.any(Object),
    )
    expect(store.items.map((item) => item.message)).toEqual([
      'hidden protocol row',
      'visible protocol row',
      'persisted protocol row',
    ])
    expect(store.selectedLogId).toBe('log_protocol_hidden_0003')
  })

  it('exposes the fixed protocol path helper', () => {
    const params = readSearchParams(buildProtocolLogListPath({
      source: 'adapter',
      limit: 50,
    }))

    expect(params.get('protocol')).toBe('onebot11')
    expect(params.get('limit')).toBe('50')
    expect(params.get('source')).toBe('adapter')
  })

  it('clears the previous detail immediately when switching to an uncached log', async () => {
    const deferred = createDeferredResponse()
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({
        log_id: 'log_protocol_detail_A',
        timestamp: '2026-04-08T10:16:00Z',
        level: 'warn',
        protocol: 'onebot11',
        source: 'adapter.onebot11',
        message: 'detail A',
        details: {
          reason: 'cached detail A',
        },
      }))
      .mockImplementationOnce(() => deferred.promise)
    vi.stubGlobal('fetch', fetchMock)

    const store = useProtocolLogsStore()
    store.activate()

    await store.selectLog('log_protocol_detail_A')
    expect(store.currentDetail?.log_id).toBe('log_protocol_detail_A')

    const pendingSelection = store.selectLog('log_protocol_detail_B')

    expect(store.selectedLogId).toBe('log_protocol_detail_B')
    expect(store.currentDetail).toBeNull()

    deferred.resolve(jsonResponse({
      log_id: 'log_protocol_detail_B',
      timestamp: '2026-04-08T10:16:01Z',
      level: 'info',
      protocol: 'onebot11',
      source: 'bridge',
      message: 'detail B',
      details: {
        reason: 'fresh detail B',
      },
    }))

    await pendingSelection

    expect(store.currentDetail?.log_id).toBe('log_protocol_detail_B')
    expect(store.currentDetail?.details.reason).toBe('fresh detail B')
  })

  it('does not restore the previous detail when the new detail request fails', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({
        log_id: 'log_protocol_detail_A',
        timestamp: '2026-04-08T10:16:00Z',
        level: 'warn',
        protocol: 'onebot11',
        source: 'adapter.onebot11',
        message: 'detail A',
        details: {
          reason: 'cached detail A',
        },
      }))
      .mockResolvedValueOnce(jsonResponse({
        error: {
          code: 'platform.internal_error',
          message: '读取详情失败',
          request_id: 'req_protocol_detail_failed',
        },
      }, 500))
    vi.stubGlobal('fetch', fetchMock)

    const store = useProtocolLogsStore()
    store.activate()

    await store.selectLog('log_protocol_detail_A')
    expect(store.currentDetail?.log_id).toBe('log_protocol_detail_A')

    await expect(store.selectLog('log_protocol_detail_B')).rejects.toMatchObject({ message: '读取详情失败' })

    expect(store.selectedLogId).toBe('log_protocol_detail_B')
    expect(store.currentDetail).toBeNull()
    expect(store.detailError).toBe('读取详情失败')
  })
})
