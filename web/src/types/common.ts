export type LogLevel = 'debug' | 'info' | 'warn' | 'error'

export type ConnectionStatus =
  | 'disconnected'
  | 'connecting'
  | 'connected'
  | 'authenticated'
  | 'auth_failed'
  | 'reconnecting'

export interface ErrorEnvelope {
  error: {
    code: string
    message: string
    message_key: string
    request_id: string
    details?: Record<string, unknown>
  }
}

export interface WebSocketFrame<T = Record<string, unknown>> {
  channel: 'logs' | 'events' | 'tasks' | 'plugin_console'
  type: string
  timestamp: string
  data: T
  request_id?: string
  error?: {
    code: string
    message?: string
    message_key: string
    details?: Record<string, unknown>
  }
}

export interface SessionExpiredFrame {
  type: 'session_expired'
  data: Record<string, never>
}
