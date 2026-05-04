import type {
  EventsPayload,
  LogSummary,
  PluginConsoleFrameData,
  TaskSummary,
  WebSocketFrame,
} from '@/types/api'
import type {
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
  let governanceRefreshHandle: ReturnType<typeof window.setTimeout> | null = null
  let governanceRefreshInFlight = false
  let governanceRefreshQueued = false
  let pendingLiveLogs: LogSummary[] = []
  let flushLiveLogsScheduled = false

  function clearPendingStatusRefresh() {
    if (statusRefreshHandle !== null) {
      window.clearTimeout(statusRefreshHandle)
      statusRefreshHandle = null
    }
    if (governanceRefreshHandle !== null) {
      window.clearTimeout(governanceRefreshHandle)
      governanceRefreshHandle = null
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

  async function runGovernanceRefresh() {
    if (governanceRefreshInFlight) {
      governanceRefreshQueued = true
      return
    }

    governanceRefreshInFlight = true
    try {
      await dependencies.governance.refresh()
    } catch {
      // governance pages keep the last successful snapshot until the next update
    } finally {
      governanceRefreshInFlight = false
      if (governanceRefreshQueued) {
        governanceRefreshQueued = false
        scheduleGovernanceRefresh()
      }
    }
  }

  function scheduleGovernanceRefresh() {
    if (governanceRefreshInFlight) {
      governanceRefreshQueued = true
      return
    }

    if (governanceRefreshHandle !== null) {
      return
    }

    governanceRefreshHandle = window.setTimeout(() => {
      governanceRefreshHandle = null
      void runGovernanceRefresh()
    }, statusRefreshDebounceMs)
  }

  function handleEventsFrame(frame: WebSocketFrame<EventsPayload>) {
    dependencies.system.applyEvent(frame.timestamp, frame.data)

    if (isServiceStatusEvent(frame.data)) {
      scheduleStatusRefresh()
      return
    }

    if (isGovernanceChangedEvent(frame.data)) {
      scheduleGovernanceRefresh()
      return
    }

    if (isPluginStateEvent(frame.data)) {
      dependencies.plugins.upsert({
        id: frame.data.plugin_id,
        registration_state: frame.data.registration_state,
        desired_state: frame.data.desired_state,
        runtime_state: frame.data.runtime_state,
        display_state: frame.data.display_state,
        commands: frame.data.commands,
        command_conflicts: frame.data.command_conflicts,
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

  function flushPendingLiveLogs() {
    flushLiveLogsScheduled = false
    if (pendingLiveLogs.length === 0) {
      return
    }
    const batch = pendingLiveLogs
    pendingLiveLogs = []
    dependencies.logs.appendBatch(batch)
    for (const log of batch) {
      dependencies.pluginConsole.appendOutboundLog(log)
    }
  }

  function scheduleFlushLiveLogs() {
    if (flushLiveLogsScheduled) {
      return
    }
    flushLiveLogsScheduled = true
    if (typeof queueMicrotask === 'function') {
      queueMicrotask(flushPendingLiveLogs)
    } else {
      Promise.resolve().then(flushPendingLiveLogs)
    }
  }

  function handleLogsFrame(frame: WebSocketFrame<LogSummary>) {
    if (frame.type === 'logs.appended') {
      pendingLiveLogs.push(frame.data)
      scheduleFlushLiveLogs()
    }
  }

  function handleConsoleFrame(frame: WebSocketFrame<PluginConsoleFrameData>) {
    if (frame.type === 'plugins.console') {
      dependencies.pluginConsole.appendConsole(frame.data)
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

function isGovernanceChangedEvent(payload: EventsPayload): payload is Extract<EventsPayload, { event_type: string }> {
  return 'event_type' in payload && payload.event_type === 'governance.changed'
}
