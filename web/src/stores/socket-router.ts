import type {
  EventsPayload,
  LogSummary,
  TaskSummary,
  WebSocketFrame,
} from '@/types/api'
import type {
  PluginConsoleFrameData,
  PluginStateEvent,
  ProtocolSnapshotEvent,
  SocketFrameRouter,
  SocketFrameRouterDependencies,
} from '@/stores/socket-types'

const statusRefreshDebounceMs = 120

export function createSocketFrameRouter(
  dependencies: SocketFrameRouterDependencies,
): SocketFrameRouter {
  let statusRefreshHandle: ReturnType<typeof window.setTimeout> | null = null
  let statusRefreshInFlight = false

  function clearPendingStatusRefresh() {
    if (statusRefreshHandle !== null) {
      window.clearTimeout(statusRefreshHandle)
      statusRefreshHandle = null
    }
  }

  async function runStatusRefresh() {
    if (statusRefreshInFlight) {
      return
    }

    statusRefreshInFlight = true
    try {
      await dependencies.system.refreshStatus()
    } catch {
      // dashboard keeps the last good snapshot until the next reconnect or manual refresh
    } finally {
      statusRefreshInFlight = false
    }
  }

  function scheduleStatusRefresh() {
    if (statusRefreshHandle !== null || statusRefreshInFlight) {
      return
    }

    statusRefreshHandle = window.setTimeout(() => {
      statusRefreshHandle = null
      void runStatusRefresh()
    }, statusRefreshDebounceMs)
  }

  function handleEventsFrame(frame: WebSocketFrame<EventsPayload>) {
    dependencies.system.applyEvent(frame.timestamp, frame.data)

    if (isServiceStatusEvent(frame.data)) {
      scheduleStatusRefresh()
      return
    }

    if (isPluginStateEvent(frame.data)) {
      dependencies.plugins.upsert({
        id: frame.data.plugin_id,
        registration_state: frame.data.registration_state,
        desired_state: frame.data.desired_state,
        runtime_state: frame.data.runtime_state,
        display_state: frame.data.display_state,
      })
      return
    }

    if (isProtocolSnapshotEvent(frame.data)) {
      dependencies.protocols.applySnapshot(frame.data.protocol_snapshot)
    }
  }

  function handleTasksFrame(frame: WebSocketFrame<TaskSummary>) {
    if (frame.type === 'tasks.updated') {
      dependencies.tasks.upsert(frame.data)
    }
  }

  function handleLogsFrame(frame: WebSocketFrame<LogSummary>) {
    if (frame.type === 'logs.appended') {
      dependencies.logs.append(frame.data)
      dependencies.plugins.appendOutboundLog(frame.data)
    }
  }

  function handleConsoleFrame(frame: WebSocketFrame<PluginConsoleFrameData>) {
    if (frame.type === 'plugins.console') {
      dependencies.plugins.appendConsole(frame.data)
    }
  }

  return {
    clearPendingStatusRefresh,
    handleEventsFrame,
    handleTasksFrame,
    handleLogsFrame,
    handleConsoleFrame,
  }
}

function isServiceStatusEvent(payload: EventsPayload): payload is Extract<EventsPayload, { service_status: string }> {
  return 'service_status' in payload
}

function isPluginStateEvent(payload: EventsPayload): payload is PluginStateEvent {
  return 'plugin_id' in payload
}

function isProtocolSnapshotEvent(payload: EventsPayload): payload is ProtocolSnapshotEvent {
  return 'protocol_snapshot' in payload
}
