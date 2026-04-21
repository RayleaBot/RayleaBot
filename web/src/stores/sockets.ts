import { defineStore } from 'pinia'

import { createSocketController } from '@/stores/socket-controller'
import { useGovernanceStore } from '@/stores/governance'
import { createSocketFrameRouter } from '@/stores/socket-router'
import { useLogsStore } from '@/stores/logs'
import { usePluginsStore } from '@/stores/plugins'
import { useProtocolsStore } from '@/stores/protocols'
import { useSessionStore } from '@/stores/session'
import { useSystemStore } from '@/stores/system'
import { useTasksStore } from '@/stores/tasks'

export const useSocketStore = defineStore('sockets', () => {
  const sessionStore = useSessionStore()
  const pluginsStore = usePluginsStore()
  const tasksStore = useTasksStore()
  const logsStore = useLogsStore()
  const governanceStore = useGovernanceStore()
  const protocolsStore = useProtocolsStore()
  const systemStore = useSystemStore()

  const router = createSocketFrameRouter({
    system: {
      applyEvent: systemStore.applyEvent,
      refreshStatus: systemStore.refreshStatus,
    },
    plugins: {
      upsert: pluginsStore.upsert,
      appendOutboundLog: pluginsStore.appendOutboundLog,
      appendConsole: pluginsStore.appendConsole,
    },
    tasks: {
      upsert: tasksStore.upsert,
    },
    logs: {
      appendBatch: logsStore.appendBatch,
    },
    governance: {
      refresh: governanceStore.refresh,
    },
    protocols: {
      applySnapshot: protocolsStore.applySnapshot,
    },
  })

  const controller = createSocketController({
    runtime: {
      getToken: () => sessionStore.token,
      onSessionExpired: (tokenSnapshot: string | null) => sessionStore.handleSessionExpired(tokenSnapshot),
    },
    router,
  })

  return {
    snapshots: controller.snapshots,
    disconnectAll: controller.disconnectAll,
    ensureManagementSockets: controller.ensureManagementSockets,
    reconnectAll: controller.reconnectAll,
    reconnectConsole: controller.reconnectConsole,
    setConsolePlugin: controller.setConsolePlugin,
  }
})
