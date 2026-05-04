import type { components } from './generated'

export type LogLevel = components['schemas']['LogLevel']
export type ErrorEnvelope = components['schemas']['ErrorEnvelope']
export type {
  ConnectionStatus,
  SessionExpiredFrame,
  WebSocketFrame,
} from './websocket.generated'
