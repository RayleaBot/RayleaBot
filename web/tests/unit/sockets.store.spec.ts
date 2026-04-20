import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

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
})
