import type { LogLevel } from './common'

export interface LogSummary {
  timestamp: string
  level: LogLevel
  source: string
  message: string
  plugin_id?: string
  request_id?: string
}

export interface LogListResponse {
  items: LogSummary[]
}
