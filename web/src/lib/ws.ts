import type { ConnectionStatus, SessionExpiredFrame, WebSocketFrame } from '@/types/api'

export interface BackoffOptions {
  baseMs: number
  capMs: number
  jitterRatio: number
}

export const DEFAULT_BACKOFF: BackoffOptions = {
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

function buildSocketUrl(path: string, token: string) {
  const configuredBase = import.meta.env.VITE_WS_BASE_URL as string | undefined
  const base = configuredBase ? new URL(configuredBase) : new URL(window.location.origin)
  const protocol = base.protocol === 'https:' ? 'wss:' : 'ws:'
  const url = new URL(path, `${protocol}//${base.host}`)
  url.searchParams.set('session_token', token)
  return url.toString()
}

export interface SocketRuntime {
  getToken: () => string | null
  onSessionExpired: (tokenSnapshot: string | null) => void
}

export interface SocketStatusDetail {
  lastError?: string
  lastErrorAt?: string
  nextBackoffMs?: number
}

export interface ManagedSocketOptions<TFrameData> {
  name: string
  path: () => string | null
  runtime: SocketRuntime
  onStatusChange?: (status: ConnectionStatus, detail: SocketStatusDetail) => void
  onFrame?: (frame: WebSocketFrame<TFrameData>) => void
  backoff?: BackoffOptions
  now?: () => Date
  random?: () => number
}

export class ManagedSocket<TFrameData = Record<string, unknown>> {
  private readonly name: string
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
  private tokenSnapshot: string | null = null
  private status: ConnectionStatus = 'disconnected'

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

  getStatus() {
    return this.status
  }

  getLastError() {
    return this.lastError
  }

  getLastErrorAt() {
    return this.lastErrorAt
  }

  getNextBackoffMs() {
    return this.nextBackoffMs
  }

  private connect() {
    const token = this.runtime.getToken()
    const path = this.getPath()

    if (!this.started || !token || !path) {
      this.tokenSnapshot = null
      this.setStatus('disconnected')
      return
    }

    this.pathSnapshot = path
    this.tokenSnapshot = token
    this.nextBackoffMs = undefined
    this.setStatus(this.reconnectAttempts > 0 ? 'reconnecting' : 'connecting')

    const socket = new WebSocket(buildSocketUrl(path, token))
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
        this.recordError(`${this.name} 收到无效消息`)
        socket.close()
        return
      }

      if ('type' in frame && frame.type === 'session_expired') {
        this.recordError('会话已失效')
        this.setStatus('auth_failed')
        this.runtime.onSessionExpired(this.tokenSnapshot)
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

      this.recordError(`${this.name} 连接异常`)
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

  private recordError(message: string) {
    this.lastError = message
    this.lastErrorAt = this.now().toISOString()
  }

  private setStatus(status: ConnectionStatus) {
    this.status = status
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
