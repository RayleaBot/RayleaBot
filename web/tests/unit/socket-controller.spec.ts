import { beforeEach, describe, expect, it, vi } from 'vitest'

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

import { createSocketController } from '@/stores/socket-controller'

describe('socket controller', () => {
  beforeEach(() => {
    MockManagedSocket.instances = []
  })

  it('starts management sockets separately and keeps snapshots public', () => {
    const controller = createSocketController({
      runtime: {
        getToken: () => 'session-token',
        onSessionExpired: vi.fn(),
      },
      router: {
        clearPendingStatusRefresh: vi.fn(),
        handleEventsFrame: vi.fn(),
        handleLogsFrame: vi.fn(),
        handleConsoleFrame: vi.fn(),
      },
    })

    controller.ensureManagementSockets()
    controller.ensureManagementSockets()

    expect(MockManagedSocket.instances).toHaveLength(3)
    expect(MockManagedSocket.instances[0].start).toHaveBeenCalledTimes(1)
    expect(MockManagedSocket.instances[1].start).toHaveBeenCalledTimes(1)
    expect(MockManagedSocket.instances[2].start).not.toHaveBeenCalled()
    expect(MockManagedSocket.instances[0].refresh).toHaveBeenCalledTimes(1)
    expect(MockManagedSocket.instances[1].refresh).toHaveBeenCalledTimes(1)

    MockManagedSocket.instances[0].emitStatus('authenticated')
    MockManagedSocket.instances[1].emitStatus('reconnecting', 'logs 连接异常')

    expect(controller.snapshots.events.status).toBe('authenticated')
    expect(controller.snapshots.logs.lastError).toBe('logs 连接异常')
  })

  it('reconnects and stops management plus console sockets independently', () => {
    const router = {
      clearPendingStatusRefresh: vi.fn(),
      handleEventsFrame: vi.fn(),
      handleLogsFrame: vi.fn(),
      handleConsoleFrame: vi.fn(),
    }
    const controller = createSocketController({
      runtime: {
        getToken: () => 'session-token',
        onSessionExpired: vi.fn(),
      },
      router,
    })

    controller.ensureManagementSockets()
    controller.setConsolePlugin('weather')
    controller.reconnectConsole()
    controller.reconnectAll()

    expect(MockManagedSocket.instances[2].start).toHaveBeenCalledTimes(3)
    expect(MockManagedSocket.instances[2].refresh).toHaveBeenCalledTimes(3)
    expect(router.clearPendingStatusRefresh).toHaveBeenCalledTimes(1)
    expect(MockManagedSocket.instances[0].refresh).toHaveBeenCalledTimes(2)
    expect(MockManagedSocket.instances[1].refresh).toHaveBeenCalledTimes(2)

    controller.setConsolePlugin(null)
    expect(MockManagedSocket.instances[2].stop).toHaveBeenCalledTimes(1)

    controller.disconnectAll()

    expect(router.clearPendingStatusRefresh).toHaveBeenCalledTimes(2)
    expect(MockManagedSocket.instances[0].stop).toHaveBeenCalledTimes(1)
    expect(MockManagedSocket.instances[1].stop).toHaveBeenCalledTimes(1)
    expect(MockManagedSocket.instances[2].stop).toHaveBeenCalledTimes(2)
  })
})
