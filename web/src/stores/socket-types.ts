import type { SocketRuntime } from '@/lib/ws'
import type {
  ConnectionStatus,
  EventsPayload,
  LogSummary,
  OneBot11ProtocolSnapshotResponse,
  PluginCommandSummary,
  PluginConsoleFrameData,
  TaskSummary,
  WebSocketFrame,
} from '@/types/api'

export type SocketChannelKey = 'events' | 'tasks' | 'logs' | 'pluginConsole'

export interface SocketSnapshot {
  status: ConnectionStatus
  lastError?: string
  lastErrorAt?: string
  nextBackoffMs?: number
}

export type SocketSnapshotMap = Record<SocketChannelKey, SocketSnapshot>

export type PluginStateEvent = Extract<EventsPayload, { plugin_id: string }>
export type ProtocolSnapshotEvent = Extract<EventsPayload, { protocol_snapshot: OneBot11ProtocolSnapshotResponse }>

export interface PluginSocketProjection {
  id: string
  registration_state: PluginStateEvent['registration_state']
  desired_state: PluginStateEvent['desired_state']
  runtime_state: PluginStateEvent['runtime_state']
  display_state: PluginStateEvent['display_state']
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
  tasks: {
    upsert: (task: TaskSummary) => void
  }
  logs: {
    appendBatch: (logs: LogSummary[]) => unknown
  }
  governance: {
    refresh: () => Promise<unknown>
  }
  protocols: {
    applySnapshot: (snapshot: OneBot11ProtocolSnapshotResponse) => void
  }
}

export interface SocketFrameRouter {
  clearPendingStatusRefresh: () => void
  handleEventsFrame: (frame: WebSocketFrame<EventsPayload>) => void
  handleTasksFrame: (frame: WebSocketFrame<TaskSummary>) => void
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
