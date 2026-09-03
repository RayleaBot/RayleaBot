import { flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useGovernanceStore } from '@/stores/governance'
import { useLogsStore } from '@/stores/logs'
import { useSchedulerJobsStore } from '@/stores/scheduler-jobs'
import { useSocketStore } from '@/stores/sockets'

const { MockManagedSocket } = vi.hoisted(() => {
  class HoistedManagedSocket<TFrameData = Record<string, unknown>> {
    static instances: HoistedManagedSocket[] = []

    readonly options: {
      name: string
      onStatusChange?: (status: string, detail: { lastError?: string; lastErrorAt?: string; nextBackoffMs?: number }) => void
      onFrame?: (frame: TFrameData) => void
    }

    start = vi.fn()
    stop = vi.fn()
    refresh = vi.fn()

    constructor(options: HoistedManagedSocket<TFrameData>['options']) {
      this.options = options
      HoistedManagedSocket.instances.push(this)
    }

    emitStatus(status: string, lastError?: string, lastErrorAt?: string, nextBackoffMs?: number) {
      this.options.onStatusChange?.(status, { lastError, lastErrorAt, nextBackoffMs })
    }
  }

  return { MockManagedSocket: HoistedManagedSocket }
})

vi.mock('@/lib/ws', () => ({
  ManagedSocket: MockManagedSocket,
}))

describe('socket store', () => {
  beforeEach(() => {
    MockManagedSocket.instances = []
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('routes live log frames through the public socket store wiring', async () => {
    const store = useSocketStore()
    const logsStore = useLogsStore()

    store.ensureManagementSockets()
    MockManagedSocket.instances[1].options.onFrame?.({
      channel: 'logs',
      type: 'logs.appended',
      timestamp: '2026-04-05T08:00:01Z',
      data: {
        log_id: 'log_0001',
        timestamp: '2026-04-05T08:00:01Z',
        level: 'info',
        source: 'runtime',
        message: 'first live row',
      },
    })

    await flushPromises()

    expect(logsStore.items.map((item) => item.log_id)).toEqual(['log_0001'])
  })

  it('routes scheduler log frames through the public socket store wiring', async () => {
    const schedulerJobsStore = useSchedulerJobsStore()
    const refreshSpy = vi.spyOn(schedulerJobsStore, 'scheduleDataSourceRefresh')
    const store = useSocketStore()

    store.ensureManagementSockets()
    MockManagedSocket.instances[1].options.onFrame?.({
      channel: 'logs',
      type: 'logs.appended',
      timestamp: '2026-05-25T08:00:01Z',
      data: {
        log_id: 'log_scheduler_0001',
        timestamp: '2026-05-25T08:00:01Z',
        level: 'info',
        source: 'scheduler',
        plugin_id: 'weather',
        message: '【天气插件｜daily_report｜每日早报｜处理成功】耗时 820ms',
      },
    })

    await flushPromises()

    expect(refreshSpy).toHaveBeenCalledTimes(1)
  })

  it('refreshes governance state through the public socket store wiring', async () => {
    vi.useFakeTimers()
    const governanceStore = useGovernanceStore()
    const refreshSpy = vi.spyOn(governanceStore, 'refresh').mockResolvedValue({
      blacklist: null,
      whitelist: null,
      commandPolicy: null,
    })
    const store = useSocketStore()

    store.ensureManagementSockets()
    MockManagedSocket.instances[0].options.onFrame?.({
      channel: 'events',
      type: 'events.received',
      timestamp: '2026-04-20T08:00:01Z',
      data: {
        event_type: 'governance.changed',
        summary: '治理设置已更新',
      },
    })

    await vi.advanceTimersByTimeAsync(120)
    await flushPromises()

    expect(refreshSpy).toHaveBeenCalledTimes(1)
  })
})
