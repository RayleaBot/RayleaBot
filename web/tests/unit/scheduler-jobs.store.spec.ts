import { flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useSchedulerJobsStore } from '@/stores/scheduler-jobs'
import type { SchedulerJobSummary } from '@/types/api'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function makeSchedulerJob(overrides: Partial<SchedulerJobSummary> = {}): SchedulerJobSummary {
  return {
    job_id: 'daily_report',
    plugin_id: 'weather',
    plugin_name: '天气插件',
    task_name: 'daily_report',
    log_label: '每日早报',
    cron_expr: '0 8 * * *',
    timezone: 'Asia/Shanghai',
    enabled: true,
    next_run: '2026-05-26T00:00:00Z',
    last_run: null,
    last_duration_ms: 0,
    payload_summary: {
      conversation_id: 'group:20001',
      target_type: 'group',
      target_id: '20001',
      content: '每日天气推送',
    },
    stats: {
      total: 0,
      success: 0,
      failed: 0,
      timeout: 0,
      retry: 0,
      other: 0,
    },
    ...overrides,
  }
}

describe('scheduler jobs store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('debounces data source driven refreshes while the page is active', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      items: [makeSchedulerJob({
        last_run: '2026-05-25T08:00:00Z',
        last_duration_ms: 820,
        stats: {
          total: 1,
          success: 1,
          failed: 0,
          timeout: 0,
          retry: 0,
          other: 0,
        },
      })],
    }))
    vi.stubGlobal('fetch', fetchMock)

    const store = useSchedulerJobsStore()
    store.setLiveRefreshActive(true)
    store.scheduleDataSourceRefresh()
    store.scheduleDataSourceRefresh()

    expect(fetchMock).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(120)
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock).toHaveBeenCalledWith('/api/system/scheduler/jobs', expect.any(Object))
    expect(store.items[0]?.last_run).toBe('2026-05-25T08:00:00Z')
    expect(store.items[0]?.stats.total).toBe(1)
  })

  it('queues one data source refresh while a list request is running', async () => {
    let resolveFirst: ((value: Response) => void) | undefined
    const fetchMock = vi.fn()
      .mockImplementationOnce(() => new Promise<Response>((resolve) => {
        resolveFirst = resolve
      }))
      .mockResolvedValueOnce(jsonResponse({
        items: [makeSchedulerJob({
          last_run: '2026-05-25T08:00:00Z',
          stats: {
            total: 1,
            success: 1,
            failed: 0,
            timeout: 0,
            retry: 0,
            other: 0,
          },
        })],
      }))
    vi.stubGlobal('fetch', fetchMock)

    const store = useSchedulerJobsStore()
    store.setLiveRefreshActive(true)
    const pending = store.fetchList()
    store.scheduleDataSourceRefresh()

    expect(fetchMock).toHaveBeenCalledTimes(1)

    resolveFirst?.(jsonResponse({ items: [makeSchedulerJob()] }))
    await pending
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(120)
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(store.items[0]?.stats.total).toBe(1)
  })

  it('ignores data source refreshes while the page is inactive', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ items: [] }))
    vi.stubGlobal('fetch', fetchMock)

    const store = useSchedulerJobsStore()
    store.scheduleDataSourceRefresh()
    await vi.advanceTimersByTimeAsync(120)

    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('cancels pending data source refreshes when the page leaves', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ items: [] }))
    vi.stubGlobal('fetch', fetchMock)

    const store = useSchedulerJobsStore()
    store.setLiveRefreshActive(true)
    store.scheduleDataSourceRefresh()
    store.setLiveRefreshActive(false)
    await vi.advanceTimersByTimeAsync(120)

    expect(fetchMock).not.toHaveBeenCalled()
  })
})
