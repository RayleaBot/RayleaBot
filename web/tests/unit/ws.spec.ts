import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  computeBackoffMs,
  DEFAULT_BACKOFF,
  ManagedSocket,
  type BackoffOptions,
  type SocketStatusDetail,
} from '@/lib/ws'

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

const deterministicRandom = () => 0.5

const fixedNow = () => new Date('2026-03-17T09:33:00Z')

function makeSocket(options: Partial<ConstructorParameters<typeof ManagedSocket>[0]> = {}) {
  return new ManagedSocket({
    name: options.name ?? 'events',
    path: options.path ?? (() => '/ws/events'),
    runtime: options.runtime ?? {
      getToken: () => 'fixture-token',
      onSessionExpired: vi.fn(),
    },
    onFrame: options.onFrame,
    onStatusChange: options.onStatusChange,
    backoff: options.backoff,
    now: options.now ?? fixedNow,
    random: options.random ?? deterministicRandom,
  })
}

describe('computeBackoffMs', () => {
  it('returns zero for non-positive attempts', () => {
    expect(computeBackoffMs(0)).toBe(0)
    expect(computeBackoffMs(-1)).toBe(0)
  })

  it('doubles delay per attempt up to the cap', () => {
    const options: BackoffOptions = { baseMs: 500, capMs: 30_000, jitterRatio: 0 }
    expect(computeBackoffMs(1, options)).toBe(500)
    expect(computeBackoffMs(2, options)).toBe(1000)
    expect(computeBackoffMs(3, options)).toBe(2000)
    expect(computeBackoffMs(4, options)).toBe(4000)
    expect(computeBackoffMs(7, options)).toBe(30_000)
    expect(computeBackoffMs(20, options)).toBe(30_000)
  })

  it('applies symmetric jitter bounded by the configured ratio', () => {
    const options: BackoffOptions = { baseMs: 1000, capMs: 30_000, jitterRatio: 0.25 }
    expect(computeBackoffMs(1, options, () => 0)).toBe(750)
    expect(computeBackoffMs(1, options, () => 1)).toBe(1250)
    expect(computeBackoffMs(1, options, () => 0.5)).toBe(1000)
  })

  it('uses the documented defaults', () => {
    expect(DEFAULT_BACKOFF.baseMs).toBe(500)
    expect(DEFAULT_BACKOFF.capMs).toBe(30_000)
    expect(DEFAULT_BACKOFF.jitterRatio).toBe(0.25)
  })
})

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
    const socket = makeSocket({ onFrame })

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
    const socket = makeSocket({
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
    const socket = makeSocket({
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
    const socket = makeSocket()

    socket.start()
    const firstInstance = FakeWebSocket.instances[0]
    firstInstance.emit('open')
    firstInstance.emit('error')
    firstInstance.emit('close')

    expect(socket.getStatus()).toBe('reconnecting')
    expect(socket.getLastError()).toBe('events 连接异常')
    expect(socket.getLastErrorAt()).toBe('2026-03-17T09:33:00.000Z')
    expect(socket.getNextBackoffMs()).toBe(500)

    vi.advanceTimersByTime(500)

    expect(FakeWebSocket.instances.length).toBe(2)
  })

  it('ignores stale close events after refresh reconnects', () => {
    let currentPath = '/ws/events'
    const socket = makeSocket({ path: () => currentPath })

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
    const socket = makeSocket()

    socket.start()
    const firstInstance = FakeWebSocket.instances[0]
    firstInstance.emit('open')
    firstInstance.emit('message', 'not-json')

    expect(socket.getLastError()).toBe('events 收到无效消息')
    expect(socket.getStatus()).toBe('reconnecting')

    vi.advanceTimersByTime(500)

    expect(FakeWebSocket.instances.length).toBe(2)
  })

  it('grows the reconnect delay exponentially and caps it', () => {
    const socket = makeSocket({
      backoff: { baseMs: 500, capMs: 4_000, jitterRatio: 0 },
    })

    socket.start()
    const expected = [500, 1_000, 2_000, 4_000, 4_000]
    for (const delay of expected) {
      const instance = FakeWebSocket.instances.at(-1)!
      instance.emit('close')
      expect(socket.getNextBackoffMs()).toBe(delay)
      vi.advanceTimersByTime(delay)
    }
  })

  it('stops scheduling reconnects after session_expired', () => {
    const onSessionExpired = vi.fn()
    const socket = makeSocket({
      runtime: { getToken: () => 'fixture-token', onSessionExpired },
    })

    socket.start()
    const instance = FakeWebSocket.instances[0]
    instance.emit('open')
    instance.emit('message', { type: 'session_expired', data: {} })

    expect(socket.getStatus()).toBe('disconnected')

    vi.advanceTimersByTime(60_000)

    expect(FakeWebSocket.instances.length).toBe(1)
  })

  it('clears lastError and nextBackoffMs once a reconnect succeeds', () => {
    const updates: Array<{ status: string; detail: SocketStatusDetail }> = []
    const socket = makeSocket({
      onStatusChange: (status, detail) => updates.push({ status, detail: { ...detail } }),
    })

    socket.start()
    const firstInstance = FakeWebSocket.instances[0]
    firstInstance.emit('open')
    firstInstance.emit('error')
    firstInstance.emit('close')
    vi.advanceTimersByTime(500)

    const secondInstance = FakeWebSocket.instances[1]
    secondInstance.emit('open')
    secondInstance.emit('message', {
      channel: 'events',
      type: 'events.received',
      timestamp: '2026-03-17T09:33:00Z',
      data: { summary: 'ready', service_status: 'running' },
    })

    expect(socket.getStatus()).toBe('authenticated')
    expect(socket.getLastError()).toBeUndefined()
    expect(socket.getLastErrorAt()).toBeUndefined()
    expect(socket.getNextBackoffMs()).toBeUndefined()

    const lastUpdate = updates.at(-1)!
    expect(lastUpdate.detail.lastError).toBeUndefined()
    expect(lastUpdate.detail.nextBackoffMs).toBeUndefined()
  })

  it('counts attempts independently across sockets', () => {
    const socketA = makeSocket({ name: 'events' })
    const socketB = makeSocket({ name: 'tasks', path: () => '/ws/tasks' })

    socketA.start()
    socketB.start()
    const aFirst = FakeWebSocket.instances[0]
    const bFirst = FakeWebSocket.instances[1]
    aFirst.emit('close')

    expect(socketA.getNextBackoffMs()).toBe(500)
    expect(socketB.getNextBackoffMs()).toBeUndefined()

    vi.advanceTimersByTime(500)
    const aSecond = FakeWebSocket.instances.at(-1)!
    aSecond.emit('close')

    expect(socketA.getNextBackoffMs()).toBe(1_000)
    expect(bFirst.readyState).toBe(FakeWebSocket.OPEN)
  })
})
