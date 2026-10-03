import { flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

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
    MockManagedSocket.instances = []
    setActivePinia(createPinia())
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
})
