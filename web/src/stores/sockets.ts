import { reactive } from 'vue'
import { defineStore } from 'pinia'

import { ManagedSocket } from '@/lib/ws'
import type { ConnectionStatus, EventsPayload, LogSummary, TaskSummary, WebSocketFrame } from '@/types/api'
import { useLogsStore } from '@/stores/logs'
import { usePluginsStore } from '@/stores/plugins'
import { useSessionStore } from '@/stores/session'
import { useSystemStore } from '@/stores/system'
import { useTasksStore } from '@/stores/tasks'

interface SocketSnapshot {
  status: ConnectionStatus
  lastError?: string
}

export const useSocketStore = defineStore('sockets', () => {
  const snapshots = reactive<Record<'events' | 'tasks' | 'logs' | 'pluginConsole', SocketSnapshot>>({
    events: { status: 'disconnected' },
    tasks: { status: 'disconnected' },
    logs: { status: 'disconnected' },
    pluginConsole: { status: 'disconnected' },
  })

  let consolePluginId: string | null = null
  let socketsInitialized = false

  const sessionStore = useSessionStore()

  const runtime = {
    getToken: () => sessionStore.token,
    onSessionExpired: () => sessionStore.handleSessionExpired(),
  }

  const pluginsStore = usePluginsStore()
  const tasksStore = useTasksStore()
  const logsStore = useLogsStore()
  const systemStore = useSystemStore()

  const eventsSocket = new ManagedSocket<EventsPayload>({
    name: 'events',
    path: () => '/ws/events',
    runtime,
    onStatusChange: (status, lastError) => {
      snapshots.events.status = status
      snapshots.events.lastError = lastError
    },
    onFrame: (frame) => {
      systemStore.applyEvent(frame.timestamp, frame.data)
      if ('plugin_id' in frame.data) {
        pluginsStore.upsert({
          id: frame.data.plugin_id,
          registration_state: frame.data.registration_state,
          desired_state: frame.data.desired_state,
          runtime_state: frame.data.runtime_state,
          display_state: frame.data.display_state,
        })
      }
    },
  })

  const tasksSocket = new ManagedSocket<TaskSummary>({
    name: 'tasks',
    path: () => '/ws/tasks',
    runtime,
    onStatusChange: (status, lastError) => {
      snapshots.tasks.status = status
      snapshots.tasks.lastError = lastError
    },
    onFrame: (frame: WebSocketFrame<TaskSummary>) => {
      if (frame.type === 'tasks.updated') {
        tasksStore.upsert(frame.data)
      }
    },
  })

  const logsSocket = new ManagedSocket<LogSummary>({
    name: 'logs',
    path: () => '/ws/logs',
    runtime,
    onStatusChange: (status, lastError) => {
      snapshots.logs.status = status
      snapshots.logs.lastError = lastError
    },
    onFrame: (frame: WebSocketFrame<LogSummary>) => {
      if (frame.type === 'logs.appended') {
        logsStore.append(frame.data)
      }
    },
  })

  const consoleSocket = new ManagedSocket<{
    plugin_id: string
    stream: 'stdout' | 'stderr' | 'system'
    text: string
    timestamp: string
  }>({
    name: 'pluginConsole',
    path: () => (consolePluginId ? `/ws/plugins/${consolePluginId}/console` : null),
    runtime,
    onStatusChange: (status, lastError) => {
      snapshots.pluginConsole.status = status
      snapshots.pluginConsole.lastError = lastError
    },
    onFrame: (frame) => {
      if (frame.type === 'plugins.console') {
        pluginsStore.appendConsole(frame.data)
      }
    },
  })

  function ensureManagementSockets() {
    if (socketsInitialized) {
      eventsSocket.refresh()
      tasksSocket.refresh()
      logsSocket.refresh()
      return
    }

    socketsInitialized = true
    eventsSocket.start()
    tasksSocket.start()
    logsSocket.start()
  }

  function disconnectAll() {
    eventsSocket.stop()
    tasksSocket.stop()
    logsSocket.stop()
    consoleSocket.stop()
    socketsInitialized = false
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

  return {
    snapshots,
    disconnectAll,
    ensureManagementSockets,
    setConsolePlugin,
  }
})
