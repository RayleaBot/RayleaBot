import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ManagedSocket } from '@/lib/ws'

class FakeWebSocket {
  static instances: FakeWebSocket[] = []

  static OPEN = 1

  readyState = FakeWebSocket.OPEN
  url: string
  listeners = new Map<string, Array<(event?: Event | MessageEvent) => void>>()

  constructor(url: string) {
    this.url = url
    FakeWebSocket.instances.push(this)
  }

  addEventListener(type: string, listener: (event?: Event | MessageEvent) => void) {
    const existing = this.listeners.get(type) ?? []
    this.listeners.set(type, [...existing, listener])
  }

  close() {
    this.emit('close')
  }

  emit(type: string, data?: unknown) {
    for (const listener of this.listeners.get(type) ?? []) {
      if (type === 'message') {
        listener({
          data: typeof data === 'string' ? data : JSON.stringify(data),
          target: this,
        } as MessageEvent)
      } else {
        listener({ target: this } as Event)
      }
    }
  }
}

describe('ManagedSocket', () => {
  beforeEach(() => {
    FakeWebSocket.instances = []
    vi.stubGlobal('WebSocket', FakeWebSocket as unknown as typeof WebSocket)
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('moves to authenticated after the first frame', () => {
    const onFrame = vi.fn()
    const socket = new ManagedSocket({
      name: 'events',
      path: () => '/ws/events',
      runtime: {
        getToken: () => 'fixture-token',
        onSessionExpired: vi.fn(),
      },
      onFrame,
    })

    socket.start()
    const instance = FakeWebSocket.instances[0]
    instance.emit('open')
    instance.emit('message', {
      channel: 'events',
      type: 'events.received',
      timestamp: '2026-03-17T09:33:00Z',
      data: {
        summary: 'ready',
        service_status: 'running',
      },
    })

    expect(socket.getStatus()).toBe('authenticated')
    expect(onFrame).toHaveBeenCalledTimes(1)
  })

  it('triggers session expiration on session_expired frame', () => {
    const onSessionExpired = vi.fn()
    const socket = new ManagedSocket({
      name: 'events',
      path: () => '/ws/events',
      runtime: {
        getToken: () => 'fixture-token',
        onSessionExpired,
      },
    })

    socket.start()
    const instance = FakeWebSocket.instances[0]
    instance.emit('open')
    instance.emit('message', {
      type: 'session_expired',
      data: {},
    })

    expect(onSessionExpired).toHaveBeenCalledTimes(1)
    expect(socket.getStatus()).toBe('disconnected')
  })

  it('reports the socket token snapshot on session expiration', () => {
    let currentToken = 'stale-token'
    const onSessionExpired = vi.fn()
    const socket = new ManagedSocket({
      name: 'events',
      path: () => '/ws/events',
      runtime: {
        getToken: () => currentToken,
        onSessionExpired,
      },
    })

    socket.start()
    const instance = FakeWebSocket.instances[0]
    instance.emit('open')
    currentToken = 'fresh-token'
    instance.emit('message', {
      type: 'session_expired',
      data: {},
    })

    expect(onSessionExpired).toHaveBeenCalledWith('stale-token')
  })

  it('records the last error and reconnects after close', () => {
    const socket = new ManagedSocket({
      name: 'events',
      path: () => '/ws/events',
      runtime: {
        getToken: () => 'fixture-token',
        onSessionExpired: vi.fn(),
      },
    })

    socket.start()
    const firstInstance = FakeWebSocket.instances[0]
    firstInstance.emit('open')
    firstInstance.emit('error')
    firstInstance.emit('close')

    expect(socket.getStatus()).toBe('reconnecting')
    expect(socket.getLastError()).toBe('events 连接异常')

    vi.advanceTimersByTime(500)

    expect(FakeWebSocket.instances.length).toBe(2)
  })

  it('ignores stale close events after refresh reconnects', () => {
    let currentPath = '/ws/events'
    const socket = new ManagedSocket({
      name: 'events',
      path: () => currentPath,
      runtime: {
        getToken: () => 'fixture-token',
        onSessionExpired: vi.fn(),
      },
    })

    socket.start()
    const firstInstance = FakeWebSocket.instances[0]
    firstInstance.emit('open')

    currentPath = '/ws/events?cursor=new'
    socket.refresh()
    const secondInstance = FakeWebSocket.instances[1]
    secondInstance.emit('open')

    firstInstance.emit('close')

    expect(socket.getStatus()).toBe('connected')
    expect(FakeWebSocket.instances.length).toBe(2)
  })

  it('closes the current socket and reconnects when a frame is not valid JSON', () => {
    const socket = new ManagedSocket({
      name: 'events',
      path: () => '/ws/events',
      runtime: {
        getToken: () => 'fixture-token',
        onSessionExpired: vi.fn(),
      },
    })

    socket.start()
    const firstInstance = FakeWebSocket.instances[0]
    firstInstance.emit('open')
    firstInstance.emit('message', 'not-json')

    expect(socket.getLastError()).toBe('events 收到无效消息')
    expect(socket.getStatus()).toBe('reconnecting')

    vi.advanceTimersByTime(500)

    expect(FakeWebSocket.instances.length).toBe(2)
  })
})
