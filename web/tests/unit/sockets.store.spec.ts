import { flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useLogsStore } from '@/stores/logs'
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
    vi.useFakeTimers()
    MockManagedSocket.instances = []
    setActivePinia(createPinia())
  })

  afterEach(() => { vi.useRealTimers() })

  it('routes live log frames through the public socket store wiring', async () => {
    const store = useSocketStore()
    const logsStore = useLogsStore()
    logsStore.setViewportActive(true)

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

    await vi.advanceTimersByTimeAsync(16)
    await flushPromises()

    expect(logsStore.items.map((item) => item.log_id)).toEqual(['log_0001'])
  })

  it('drops the old connection batch before accepting frames from a new connection', async () => {
    const store = useSocketStore()
    const logsStore = useLogsStore()
    logsStore.setViewportActive(true)
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ items: [], page: { has_older: false } }), { headers: { 'Content-Type': 'application/json' } })))
    const logs = MockManagedSocket.instances[1]!
    const frame = (id: string) => ({ channel: 'logs', type: 'logs.appended', timestamp: '2026-04-05T08:00:01Z', data: { log_id: id, timestamp: '2026-04-05T08:00:01Z', level: 'info', source: 'runtime', message: id } })
    logs.options.onFrame?.(frame('old'))
    logs.emitStatus('reconnecting')
    logs.emitStatus('connected')
    logs.options.onFrame?.(frame('new'))
    await vi.advanceTimersByTimeAsync(16)
    await flushPromises()
    expect(logsStore.items.map(log => log.log_id)).toEqual(['new'])
    store.disconnectAll()
    logsStore.$dispose()
  })
})
