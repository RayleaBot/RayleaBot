import { reactive } from 'vue'

import { ManagedSocket, type SocketStatusDetail } from '@/lib/ws'
import type {
  EventsPayload,
  LogSummary,
  TaskSummary,
} from '@/types/api'
import type {
  SocketChannelKey,
  SocketController,
  SocketControllerOptions,
  SocketSnapshotMap,
} from '@/stores/socket-types'
import type { PluginConsoleFrameData } from '@/types/api'

export function createSocketController(options: SocketControllerOptions): SocketController {
  const snapshots = reactive<SocketSnapshotMap>({
    events: { status: 'disconnected' },
    tasks: { status: 'disconnected' },
    logs: { status: 'disconnected' },
    pluginConsole: { status: 'disconnected' },
  })

  let consolePluginId: string | null = null
  let managementSocketsStarted = false

  const eventsSocket = new ManagedSocket<EventsPayload>({
    name: 'events',
    path: () => '/ws/events',
    runtime: options.runtime,
    onStatusChange: createSnapshotUpdater(snapshots, 'events'),
    onFrame: options.router.handleEventsFrame,
  })

  const tasksSocket = new ManagedSocket<TaskSummary>({
    name: 'tasks',
    path: () => '/ws/tasks',
    runtime: options.runtime,
    onStatusChange: createSnapshotUpdater(snapshots, 'tasks'),
    onFrame: options.router.handleTasksFrame,
  })

  const logsSocket = new ManagedSocket<LogSummary>({
    name: 'logs',
    path: () => '/ws/logs',
    runtime: options.runtime,
    onStatusChange: createSnapshotUpdater(snapshots, 'logs'),
    onFrame: options.router.handleLogsFrame,
  })

  const consoleSocket = new ManagedSocket<PluginConsoleFrameData>({
    name: 'pluginConsole',
    path: () => (consolePluginId ? `/ws/plugins/${consolePluginId}/console` : null),
    runtime: options.runtime,
    onStatusChange: createSnapshotUpdater(snapshots, 'pluginConsole'),
    onFrame: options.router.handleConsoleFrame,
  })

  function refreshManagementSockets() {
    eventsSocket.refresh()
    tasksSocket.refresh()
    logsSocket.refresh()
  }

  function ensureManagementSockets() {
    if (managementSocketsStarted) {
      refreshManagementSockets()
      return
    }

    managementSocketsStarted = true
    eventsSocket.start()
    tasksSocket.start()
    logsSocket.start()
  }

  function disconnectAll() {
    options.router.clearPendingStatusRefresh()
    eventsSocket.stop()
    tasksSocket.stop()
    logsSocket.stop()
    consoleSocket.stop()
    managementSocketsStarted = false
  }

  function reconnectAll() {
    options.router.clearPendingStatusRefresh()
    ensureManagementSockets()
    refreshManagementSockets()

    if (consolePluginId) {
      consoleSocket.start()
      consoleSocket.refresh()
    }
  }

  function setConsolePlugin(pluginId: string | null) {
    consolePluginId = pluginId
    if (pluginId) {
      consoleSocket.start()
      consoleSocket.refresh()
      return
    }

    consoleSocket.stop()
  }

  function reconnectConsole() {
    if (!consolePluginId) {
      return
    }

    consoleSocket.start()
    consoleSocket.refresh()
  }

  return {
    snapshots,
    disconnectAll,
    ensureManagementSockets,
    reconnectAll,
    reconnectConsole,
    setConsolePlugin,
  }
}

function createSnapshotUpdater(
  snapshots: SocketSnapshotMap,
  channel: SocketChannelKey,
) {
  return (status: SocketSnapshotMap[SocketChannelKey]['status'], detail: SocketStatusDetail) => {
    const target = snapshots[channel]
    target.status = status
    target.lastError = detail.lastError
    target.lastErrorAt = detail.lastErrorAt
    target.nextBackoffMs = detail.nextBackoffMs
  }
}
