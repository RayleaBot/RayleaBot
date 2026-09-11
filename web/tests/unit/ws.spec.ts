import { t } from '@/i18n'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  computeBackoffMs,
  ManagedSocket,
  type BackoffOptions,
  type SocketStatusDetail,
} from '@/lib/ws'
import type { ConnectionStatus } from '@/types/api'

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

interface SocketUpdate {
  status: ConnectionStatus
  detail: SocketStatusDetail
}

function latestUpdate(updates: SocketUpdate[]) {
  const update = updates.at(-1)
  if (!update) {
    throw new Error('expected a socket status update')
  }
  return update
}

function makeSocket(options: Partial<ConstructorParameters<typeof ManagedSocket>[0]> = {}) {
  const updates: SocketUpdate[] = []
  const onStatusChange = options.onStatusChange
  const socket = new ManagedSocket({
    name: options.name ?? 'events',
    path: options.path ?? (() => '/ws/events'),
    runtime: options.runtime ?? {
      isAuthenticated: () => true,
      onSessionExpired: vi.fn(),
    },
    onFrame: options.onFrame,
    onStatusChange: (status, detail) => {
      updates.push({ status, detail: { ...detail } })
      onStatusChange?.(status, detail)
    },
    backoff: options.backoff,
    now: options.now ?? fixedNow,
    random: options.random ?? deterministicRandom,
  })

  return { socket, updates }
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
    const { socket, updates } = makeSocket({ onFrame })

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

    expect(latestUpdate(updates).status).toBe('authenticated')
    expect(onFrame).toHaveBeenCalledTimes(1)
  })

  it('triggers session expiration on session_expired frame', () => {
    const onSessionExpired = vi.fn()
    const { socket, updates } = makeSocket({
      runtime: {
        isAuthenticated: () => true,
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
    expect(latestUpdate(updates).status).toBe('disconnected')
  })

  it('does not put a bearer token in the WebSocket URL', () => {
    const onSessionExpired = vi.fn()
    const { socket } = makeSocket({
      runtime: {
        isAuthenticated: () => true,
        onSessionExpired,
      },
    })

    socket.start()
    const instance = FakeWebSocket.instances[0]
    instance.emit('open')

    const url = new URL(instance.url)
    expect(url.searchParams.has('session_token')).toBe(false)
    expect(url.pathname).toBe('/ws/events')
  })

  it('records the last error and reconnects after close', () => {
    const { socket, updates } = makeSocket()

    socket.start()
    const firstInstance = FakeWebSocket.instances[0]
    firstInstance.emit('open')
    firstInstance.emit('error')
    firstInstance.emit('close')

    expect(latestUpdate(updates)).toEqual({
      status: 'reconnecting',
      detail: {
        lastError: t('display.connectionErrors.connectionFailed', { channel: t('display.connectionChannels.events') }),
        lastErrorAt: '2026-03-17T09:33:00.000Z',
        nextBackoffMs: 500,
      },
    })

    vi.advanceTimersByTime(500)

    expect(FakeWebSocket.instances.length).toBe(2)
  })

  it('ignores stale close events after refresh reconnects', () => {
    let currentPath = '/ws/events'
    const { socket, updates } = makeSocket({ path: () => currentPath })

    socket.start()
    const firstInstance = FakeWebSocket.instances[0]
    firstInstance.emit('open')

    currentPath = '/ws/events?cursor=new'
    socket.refresh()
    const secondInstance = FakeWebSocket.instances[1]
    secondInstance.emit('open')

    firstInstance.emit('close')

    expect(latestUpdate(updates).status).toBe('connected')
    expect(FakeWebSocket.instances.length).toBe(2)
  })

  it('closes the current socket and reconnects when a frame is not valid JSON', () => {
    const { socket, updates } = makeSocket()

    socket.start()
    const firstInstance = FakeWebSocket.instances[0]
    firstInstance.emit('open')
    firstInstance.emit('message', 'not-json')

    expect(latestUpdate(updates).detail.lastError).toBe(t('display.connectionErrors.invalidMessage', { channel: t('display.connectionChannels.events') }))
    expect(latestUpdate(updates).status).toBe('reconnecting')

    vi.advanceTimersByTime(500)

    expect(FakeWebSocket.instances.length).toBe(2)
  })

  it('grows the reconnect delay exponentially and caps it', () => {
    const { socket, updates } = makeSocket({
      backoff: { baseMs: 500, capMs: 4_000, jitterRatio: 0 },
    })

    socket.start()
    const expected = [500, 1_000, 2_000, 4_000, 4_000]
    for (const delay of expected) {
      const instance = FakeWebSocket.instances.at(-1)!
      instance.emit('close')
      expect(latestUpdate(updates).detail.nextBackoffMs).toBe(delay)
      vi.advanceTimersByTime(delay)
    }
  })

  it('stops scheduling reconnects after session_expired', () => {
    const onSessionExpired = vi.fn()
    const { socket, updates } = makeSocket({
      runtime: { isAuthenticated: () => true, onSessionExpired },
    })

    socket.start()
    const instance = FakeWebSocket.instances[0]
    instance.emit('open')
    instance.emit('message', { type: 'session_expired', data: {} })

    expect(latestUpdate(updates).status).toBe('disconnected')

    vi.advanceTimersByTime(60_000)

    expect(FakeWebSocket.instances.length).toBe(1)
  })

  it('clears lastError and nextBackoffMs once a reconnect succeeds', () => {
    const { socket, updates } = makeSocket()

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

    const lastUpdate = latestUpdate(updates)
    expect(lastUpdate.status).toBe('authenticated')
    expect(lastUpdate.detail.lastError).toBeUndefined()
    expect(lastUpdate.detail.lastErrorAt).toBeUndefined()
    expect(lastUpdate.detail.nextBackoffMs).toBeUndefined()
  })

  it('counts attempts independently across sockets', () => {
    const { socket: socketA, updates: updatesA } = makeSocket({ name: 'events' })
    const { socket: socketB, updates: updatesB } = makeSocket({ name: 'logs', path: () => '/ws/logs' })

    socketA.start()
    socketB.start()
    const aFirst = FakeWebSocket.instances[0]
    const bFirst = FakeWebSocket.instances[1]
    aFirst.emit('close')

    expect(latestUpdate(updatesA).detail.nextBackoffMs).toBe(500)
    expect(latestUpdate(updatesB).detail.nextBackoffMs).toBeUndefined()

    vi.advanceTimersByTime(500)
    const aSecond = FakeWebSocket.instances.at(-1)!
    aSecond.emit('close')

    expect(latestUpdate(updatesA).detail.nextBackoffMs).toBe(1_000)
    expect(bFirst.readyState).toBe(FakeWebSocket.OPEN)
  })
})
