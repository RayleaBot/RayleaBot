import { t } from '@/i18n'
import { webSocketEvents, webSocketPaths } from '@/types/websocket.generated'
import type { ConnectionStatus, SessionExpiredFrame, WebSocketFrame } from '@/types/api'

export interface BackoffOptions {
  baseMs: number
  capMs: number
  jitterRatio: number
}

const DEFAULT_BACKOFF: BackoffOptions = {
  baseMs: 500,
  capMs: 30_000,
  jitterRatio: 0.25,
}

export function computeBackoffMs(
  attempts: number,
  options: BackoffOptions = DEFAULT_BACKOFF,
  random: () => number = Math.random,
): number {
  if (attempts <= 0) {
    return 0
  }
  const exponent = Math.min(attempts - 1, 30)
  const exponential = options.baseMs * 2 ** exponent
  const capped = Math.min(options.capMs, exponential)
  const jitter = options.jitterRatio > 0 ? (random() * 2 - 1) * options.jitterRatio : 0
  const withJitter = capped * (1 + jitter)
  return Math.max(0, Math.round(withJitter))
}

function buildSocketUrl(path: string) {
  const configuredBase = import.meta.env.VITE_WS_BASE_URL as string | undefined
  const base = configuredBase ? new URL(configuredBase) : new URL(window.location.origin)
  const protocol = base.protocol === 'https:' ? 'wss:' : 'ws:'
  const url = new URL(path, `${protocol}//${base.host}`)
  return url.toString()
}

export interface SocketRuntime {
  isAuthenticated: () => boolean
  onSessionExpired: () => void
}

export interface SocketStatusDetail {
  lastError?: string
  lastErrorAt?: string
  nextBackoffMs?: number
}

export interface ManagedSocketOptions<TFrameData> {
  name: keyof typeof webSocketPaths
  path: () => string | null
  runtime: SocketRuntime
  onStatusChange?: (status: ConnectionStatus, detail: SocketStatusDetail) => void
  onFrame?: (frame: WebSocketFrame<TFrameData>) => void
  backoff?: BackoffOptions
  now?: () => Date
  random?: () => number
}

export class ManagedSocket<TFrameData = Record<string, unknown>> {
  private readonly name: keyof typeof webSocketPaths
  private readonly getPath: () => string | null
  private readonly runtime: SocketRuntime
  private readonly onStatusChange?: (status: ConnectionStatus, detail: SocketStatusDetail) => void
  private readonly onFrame?: (frame: WebSocketFrame<TFrameData>) => void
  private readonly backoff: BackoffOptions
  private readonly now: () => Date
  private readonly random: () => number

  private socket: WebSocket | null = null
  private reconnectHandle: number | null = null
  private reconnectAttempts = 0
  private started = false
  private lastError: string | undefined
  private lastErrorAt: string | undefined
  private nextBackoffMs: number | undefined
  private pathSnapshot: string | null = null

  constructor(options: ManagedSocketOptions<TFrameData>) {
    this.name = options.name
    this.getPath = options.path
    this.runtime = options.runtime
    this.onStatusChange = options.onStatusChange
    this.onFrame = options.onFrame
    this.backoff = options.backoff ?? DEFAULT_BACKOFF
    this.now = options.now ?? (() => new Date())
    this.random = options.random ?? Math.random
  }

  start() {
    this.started = true
    this.connect()
  }

  stop() {
    this.started = false
    this.clearReconnect()
    this.close('disconnected')
  }

  refresh() {
    const nextPath = this.getPath()
    if (nextPath === this.pathSnapshot && this.socket?.readyState === WebSocket.OPEN) {
      return
    }

    this.clearReconnect()
    this.close('disconnected')
    if (this.started) {
      this.connect()
    }
  }

  private connect() {
    const path = this.getPath()

    if (!this.started || !this.runtime.isAuthenticated() || !path) {
      this.setStatus('disconnected')
      return
    }

    this.pathSnapshot = path
    this.nextBackoffMs = undefined
    this.setStatus(this.reconnectAttempts > 0 ? 'reconnecting' : 'connecting')

    const socket = new WebSocket(buildSocketUrl(path))
    this.socket = socket

    socket.addEventListener('open', () => {
      if (this.socket !== socket) {
        return
      }

      this.reconnectAttempts = 0
      this.nextBackoffMs = undefined
      this.setStatus('connected')
    })

    socket.addEventListener('message', (event) => {
      if (this.socket !== socket) {
        return
      }

      let frame: WebSocketFrame<TFrameData> | SessionExpiredFrame
      try {
        frame = JSON.parse(String(event.data)) as WebSocketFrame<TFrameData> | SessionExpiredFrame
      } catch {
        this.recordError(t('display.connectionErrors.invalidMessage', { channel: this.channelLabel() }))
        socket.close()
        return
      }

      if ('type' in frame && frame.type === webSocketEvents.sessionExpired) {
        this.recordError(t('display.connectionErrors.sessionExpired'))
        this.setStatus('auth_failed')
        this.runtime.onSessionExpired()
        this.stop()
        return
      }

      this.setStatus('authenticated')
      this.onFrame?.(frame as WebSocketFrame<TFrameData>)
    })

    socket.addEventListener('error', () => {
      if (this.socket !== socket) {
        return
      }

      this.recordError(t('display.connectionErrors.connectionFailed', { channel: this.channelLabel() }))
    })

    socket.addEventListener('close', () => {
      if (this.socket !== socket) {
        return
      }

      this.socket = null
      if (!this.started) {
        this.setStatus('disconnected')
        return
      }

      this.scheduleReconnect()
    })
  }

  private close(nextStatus: ConnectionStatus) {
    if (this.socket) {
      const socket = this.socket
      this.socket = null
      socket.close()
    }
    this.setStatus(nextStatus)
  }

  private scheduleReconnect() {
    this.reconnectAttempts += 1
    const delay = computeBackoffMs(this.reconnectAttempts, this.backoff, this.random)
    this.nextBackoffMs = delay
    this.setStatus('reconnecting')
    this.reconnectHandle = window.setTimeout(() => {
      this.reconnectHandle = null
      this.connect()
    }, delay)
  }

  private clearReconnect() {
    if (this.reconnectHandle !== null) {
      window.clearTimeout(this.reconnectHandle)
      this.reconnectHandle = null
    }
  }

  private channelLabel() {
    return t(`display.connectionChannels.${this.name}`)
  }

  private recordError(message: string) {
    this.lastError = message
    this.lastErrorAt = this.now().toISOString()
  }

  private setStatus(status: ConnectionStatus) {
    if (status === 'authenticated' || status === 'connected') {
      this.lastError = undefined
      this.lastErrorAt = undefined
    }
    if (status !== 'reconnecting') {
      this.nextBackoffMs = undefined
    }

    this.onStatusChange?.(status, {
      lastError: this.lastError,
      lastErrorAt: this.lastErrorAt,
      nextBackoffMs: this.nextBackoffMs,
    })
  }
}
