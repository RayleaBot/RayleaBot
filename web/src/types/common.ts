import type { components } from './generated'

// --- Contract-derived (from contracts/web-api.openapi.yaml) ---
export type LogLevel = components['schemas']['LogLevel']
export type ErrorEnvelope = components['schemas']['ErrorEnvelope']

// --- WebSocket-only (from contracts/websocket-events.yaml, not in HTTP OpenAPI) ---
export type ConnectionStatus =
  | 'disconnected'
  | 'connecting'
  | 'connected'
  | 'authenticated'
  | 'auth_failed'
  | 'reconnecting'

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
