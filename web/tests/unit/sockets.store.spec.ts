import { flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useGovernanceStore } from '@/stores/governance'
import { useLogsStore } from '@/stores/logs'
import { useSessionStore } from '@/stores/session'
import { useSocketStore } from '@/stores/sockets'

const { MockManagedSocket } = vi.hoisted(() => {
  class HoistedManagedSocket<TFrameData = Record<string, unknown>> {
    static instances: HoistedManagedSocket[] = []

    readonly options: {
      name: string
      onStatusChange?: (status: string, lastError?: string) => void
      onFrame?: (frame: TFrameData) => void
    }

    start = vi.fn()
    stop = vi.fn()
    refresh = vi.fn()

    constructor(options: HoistedManagedSocket<TFrameData>['options']) {
      this.options = options
      HoistedManagedSocket.instances.push(this)
    }

    emitStatus(status: string, lastError?: string) {
      this.options.onStatusChange?.(status, lastError)
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

  it('starts management sockets once and keeps snapshots public', () => {
    const sessionStore = useSessionStore()
    sessionStore.token = 'session-token'
    const store = useSocketStore()

    store.ensureManagementSockets()
    store.ensureManagementSockets()

    expect(MockManagedSocket.instances).toHaveLength(4)
    expect(MockManagedSocket.instances[0].start).toHaveBeenCalledTimes(1)
    expect(MockManagedSocket.instances[1].start).toHaveBeenCalledTimes(1)
    expect(MockManagedSocket.instances[2].start).toHaveBeenCalledTimes(1)
    expect(MockManagedSocket.instances[3].start).not.toHaveBeenCalled()

    MockManagedSocket.instances[0].emitStatus('authenticated')
    MockManagedSocket.instances[1].emitStatus('reconnecting', 'tasks 连接异常')

    expect(store.snapshots.events.status).toBe('authenticated')
    expect(store.snapshots.tasks.lastError).toBe('tasks 连接异常')
  })

  it('keeps console and reconnect controls stable', () => {
    const sessionStore = useSessionStore()
    sessionStore.token = 'session-token'
    const store = useSocketStore()

    store.ensureManagementSockets()
    store.setConsolePlugin('weather')
    store.reconnectConsole()
    store.reconnectAll()

    expect(MockManagedSocket.instances[3].start).toHaveBeenCalledTimes(3)
    expect(MockManagedSocket.instances[3].refresh).toHaveBeenCalledTimes(3)

    store.setConsolePlugin(null)
    expect(MockManagedSocket.instances[3].stop).toHaveBeenCalledTimes(1)

    store.disconnectAll()

    expect(MockManagedSocket.instances[0].stop).toHaveBeenCalledTimes(1)
    expect(MockManagedSocket.instances[1].stop).toHaveBeenCalledTimes(1)
    expect(MockManagedSocket.instances[2].stop).toHaveBeenCalledTimes(1)
    expect(MockManagedSocket.instances[3].stop).toHaveBeenCalledTimes(2)
  })

  it('routes live log frames through the public socket store wiring', async () => {
    const sessionStore = useSessionStore()
    sessionStore.token = 'session-token'
    const store = useSocketStore()
    const logsStore = useLogsStore()

    store.ensureManagementSockets()
    MockManagedSocket.instances[2].options.onFrame?.({
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

  it('refreshes governance state through the public socket store wiring', async () => {
    vi.useFakeTimers()
    const sessionStore = useSessionStore()
    sessionStore.token = 'session-token'
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
