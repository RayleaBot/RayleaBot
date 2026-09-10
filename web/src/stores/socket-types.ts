import type { SocketRuntime } from '@/lib/ws'
import type {
  ConnectionStatus,
  EventsPayload,
  LogSummary,
  PluginCommandSummary,
  PluginConsoleFrameData,
  WebSocketFrame,
} from '@/types/api'

export type SocketChannelKey = 'events' | 'logs' | 'pluginConsole'

export interface SocketSnapshot {
  status: ConnectionStatus
  lastError?: string
  lastErrorAt?: string
  nextBackoffMs?: number
}

export type SocketSnapshotMap = Record<SocketChannelKey, SocketSnapshot>

export type PluginStateEvent = Extract<EventsPayload, { plugin_id: string }>
export type AdaptersSnapshotEvent = Extract<EventsPayload, { adapters: unknown }>

export interface PluginSocketProjection {
  id: string
  state: PluginStateEvent['state']
  state_diagnosis?: PluginStateEvent['state_diagnosis']
  commands?: PluginCommandSummary[]
  command_conflicts?: string[]
}

export interface SocketFrameRouterDependencies {
  system: {
    applyEvent: (timestamp: string, payload: EventsPayload) => void
    refreshStatus: () => Promise<unknown>
  }
  plugins: {
    upsert: (plugin: PluginSocketProjection) => void
  }
  pluginConsole: {
    appendOutboundLog: (log: LogSummary) => void
    appendConsole: (frame: PluginConsoleFrameData) => void
  }
  schedulerJobs: {
    scheduleDataSourceRefresh: () => void
  }
  logs: {
    appendBatch: (logs: LogSummary[]) => unknown
  }
  governance: {
    refresh: () => Promise<unknown>
  }
  thirdPartyAccounts: {
    refresh: () => Promise<unknown>
  }
  adapters: {
    applySnapshot: (adapters: AdaptersSnapshotEvent['adapters']) => void
  }
}

export interface SocketFrameRouter {
  clearPendingStatusRefresh: () => void
  handleEventsFrame: (frame: WebSocketFrame<EventsPayload>) => void
  handleLogsFrame: (frame: WebSocketFrame<LogSummary>) => void
  handleConsoleFrame: (frame: WebSocketFrame<PluginConsoleFrameData>) => void
}

export interface SocketControllerOptions {
  runtime: SocketRuntime
  router: SocketFrameRouter
}

export interface SocketController {
  snapshots: SocketSnapshotMap
  disconnectAll: () => void
  ensureManagementSockets: () => void
  reconnectAll: () => void
  reconnectConsole: () => void
  setConsolePlugin: (pluginId: string | null) => void
}
