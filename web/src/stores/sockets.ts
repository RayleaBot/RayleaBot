import { defineStore } from 'pinia'
import { onScopeDispose, watch } from 'vue'
import { useConfigStore } from '@/stores/config'

import { createSocketController } from '@/stores/socket-controller'
import { useGovernanceStore } from '@/stores/governance'
import { createSocketFrameRouter } from '@/stores/socket-router'
import { useLogsStore } from '@/stores/logs'
import { usePluginConsoleStore } from '@/stores/plugin-console'
import { usePluginsStore } from '@/stores/plugins'
import { useAdaptersStore } from '@/stores/adapters'
import { useSessionStore } from '@/stores/session'
import { useSchedulerJobsStore } from '@/stores/scheduler-jobs'
import { useSystemStore } from '@/stores/system'

export const useSocketStore = defineStore('sockets', () => {
  const sessionStore = useSessionStore()
  const configStore = useConfigStore()
  const pluginsStore = usePluginsStore()
  const pluginConsoleStore = usePluginConsoleStore()
  const schedulerJobsStore = useSchedulerJobsStore()
  const logsStore = useLogsStore()
  const governanceStore = useGovernanceStore()
  const adaptersStore = useAdaptersStore()
  const systemStore = useSystemStore()

  const router = createSocketFrameRouter({
    system: {
      applyEvent: systemStore.applyEvent,
      refreshStatus: systemStore.refreshStatus,
    },
    plugins: {
      upsert: pluginsStore.upsert,
      cancelPendingRefresh: pluginsStore.cancelDataSourceRefresh,
    },
    pluginConsole: {
      appendOutboundLog: pluginConsoleStore.appendOutboundLog,
      appendConsole: pluginConsoleStore.appendConsole,
    },
    schedulerJobs: {
      scheduleDataSourceRefresh: schedulerJobsStore.scheduleDataSourceRefresh,
      cancelPendingRefresh: schedulerJobsStore.cancelDataSourceRefresh,
    },
    logs: {
      appendBatch: logsStore.appendBatch,
    },
    governance: {
      refresh: governanceStore.refresh,
    },
    adapters: {
      applySnapshot: adaptersStore.applySnapshot,
    },
  })

  const controller = createSocketController({
    runtime: {
      isAuthenticated: () => sessionStore.isAuthenticated,
      onSessionExpired: () => sessionStore.handleSessionExpired(),
    },
    router,
  })

  watch(() => controller.snapshots.events.status, status => {
    if (status === 'authenticated') void configStore.refreshEffectiveTimezone().catch(() => undefined)
  })

  onScopeDispose(controller.disconnectAll)

  return {
    snapshots: controller.snapshots,
    disconnectAll: controller.disconnectAll,
    ensureManagementSockets: controller.ensureManagementSockets,
    reconnectAll: controller.reconnectAll,
    reconnectConsole: controller.reconnectConsole,
    setConsolePlugin: controller.setConsolePlugin,
  }
})
