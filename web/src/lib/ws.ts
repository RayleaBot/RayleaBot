import type { ConnectionStatus, SessionExpiredFrame, WebSocketFrame } from '@/types/api'

const reconnectDelays = [500, 1000, 2000, 4000]

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
  onSessionExpired: () => void
}

export interface ManagedSocketOptions<TFrameData> {
  name: string
  path: () => string | null
  runtime: SocketRuntime
  onStatusChange?: (status: ConnectionStatus, lastError?: string) => void
  onFrame?: (frame: WebSocketFrame<TFrameData>) => void
}

export class ManagedSocket<TFrameData = Record<string, unknown>> {
  private readonly name: string
  private readonly getPath: () => string | null
  private readonly runtime: SocketRuntime
  private readonly onStatusChange?: (status: ConnectionStatus, lastError?: string) => void
  private readonly onFrame?: (frame: WebSocketFrame<TFrameData>) => void

  private socket: WebSocket | null = null
  private reconnectHandle: number | null = null
  private reconnectAttempts = 0
  private started = false
  private lastError: string | undefined
  private pathSnapshot: string | null = null
  private status: ConnectionStatus = 'disconnected'

  constructor(options: ManagedSocketOptions<TFrameData>) {
    this.name = options.name
    this.getPath = options.path
    this.runtime = options.runtime
    this.onStatusChange = options.onStatusChange
    this.onFrame = options.onFrame
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

  private connect() {
    const token = this.runtime.getToken()
    const path = this.getPath()

    if (!this.started || !token || !path) {
      this.setStatus('disconnected')
      return
    }

    this.pathSnapshot = path
    this.setStatus(this.reconnectAttempts > 0 ? 'reconnecting' : 'connecting')

    const socket = new WebSocket(buildSocketUrl(path, token))
    this.socket = socket

    socket.addEventListener('open', () => {
      this.reconnectAttempts = 0
      this.setStatus('connected')
    })

    socket.addEventListener('message', (event) => {
      const frame = JSON.parse(String(event.data)) as WebSocketFrame<TFrameData> | SessionExpiredFrame

      if ('type' in frame && frame.type === 'session_expired') {
        this.setStatus('auth_failed', 'session expired')
        this.runtime.onSessionExpired()
        this.stop()
        return
      }

      this.setStatus('authenticated')
      this.onFrame?.(frame as WebSocketFrame<TFrameData>)
    })

    socket.addEventListener('error', () => {
      this.lastError = `${this.name} socket error`
    })

    socket.addEventListener('close', () => {
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
    const delay = reconnectDelays[Math.min(this.reconnectAttempts - 1, reconnectDelays.length - 1)]
    this.setStatus('reconnecting', this.lastError)
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

  private setStatus(status: ConnectionStatus, error?: string) {
    this.status = status
    if (error) {
      this.lastError = error
    } else if (status === 'authenticated' || status === 'connected') {
      this.lastError = undefined
    }

    this.onStatusChange?.(status, this.lastError)
  }
}
