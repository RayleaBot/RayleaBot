import { webSocketEvents, managementEventTypes } from '@/types/websocket.generated'
import { createRefreshScheduler } from '@/lib/refresh-scheduler'
import type {
  EventsPayload,
  LogSummary,
  PluginConsoleFrameData,
  WebSocketFrame,
} from '@/types/api'
import type {
  AdaptersSnapshotEvent,
  PluginStateEvent,
  SocketFrameRouter,
  SocketFrameRouterDependencies,
} from '@/stores/socket-types'

const statusRefreshDebounceMs = 120

export function createSocketFrameRouter(
  dependencies: SocketFrameRouterDependencies,
): SocketFrameRouter {
  const statusRefresh = createRefreshScheduler(dependencies.system.refreshStatus, statusRefreshDebounceMs)
  const governanceRefresh = createRefreshScheduler(dependencies.governance.refresh, statusRefreshDebounceMs)
  let pendingLiveLogs: LogSummary[] = []
  let flushLiveLogsScheduled = false

  function clearPendingStatusRefresh() {
    statusRefresh.cancel()
    governanceRefresh.cancel()
    dependencies.schedulerJobs.cancelPendingRefresh?.()
    dependencies.plugins.cancelPendingRefresh?.()
    pendingLiveLogs = []
  }

  function handleEventsFrame(frame: WebSocketFrame<EventsPayload>) {
    if (frame.type !== webSocketEvents.eventsReceived) return
    dependencies.system.applyEvent(frame.timestamp, frame.data)

    if (isServiceStatusEvent(frame.data)) {
      statusRefresh.schedule()
      return
    }

    if (isGovernanceChangedEvent(frame.data)) {
      governanceRefresh.schedule()
      return
    }

    if (isPluginStateEvent(frame.data)) {
      dependencies.plugins.upsert({
        id: frame.data.plugin_id,
        state: frame.data.state,
        state_diagnosis: frame.data.state_diagnosis,
        commands: frame.data.commands,
        command_conflicts: frame.data.command_conflicts,
      })
      dependencies.schedulerJobs.scheduleDataSourceRefresh()
      return
    }

    // The adapters listing covers every configured instance, including one
    // whose protocol has no transport snapshot of its own.
    if (isAdaptersSnapshotEvent(frame.data)) {
      dependencies.adapters.applySnapshot(frame.data.adapters)
      statusRefresh.schedule()
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
    if (frame.type === webSocketEvents.logsAppended) {
      pendingLiveLogs.push(frame.data)
      scheduleFlushLiveLogs()
      if (isSchedulerLog(frame.data)) {
        dependencies.schedulerJobs.scheduleDataSourceRefresh()
      }
    }
  }

  function handleConsoleFrame(frame: WebSocketFrame<PluginConsoleFrameData>) {
    if (frame.type === webSocketEvents.pluginsConsole) {
      dependencies.pluginConsole.appendConsole(frame.data)
    }
  }

  return {
    clearPendingStatusRefresh,
    handleEventsFrame,
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

function isAdaptersSnapshotEvent(payload: EventsPayload): payload is AdaptersSnapshotEvent {
  return 'adapters' in payload
}

function isGovernanceChangedEvent(payload: EventsPayload): payload is Extract<EventsPayload, { event_type: string }> {
  return 'event_type' in payload && payload.event_type === managementEventTypes.governanceChanged
}

function isSchedulerLog(log: LogSummary) {
  return log.source === 'scheduler'
}
