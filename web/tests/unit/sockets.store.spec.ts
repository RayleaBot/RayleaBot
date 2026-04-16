import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useLogsStore } from '@/stores/logs'
import { usePluginsStore } from '@/stores/plugins'
import { useSessionStore } from '@/stores/session'
import { useSocketStore } from '@/stores/sockets'
import { useSystemStore } from '@/stores/system'
import { useTasksStore } from '@/stores/tasks'

const { MockManagedSocket } = vi.hoisted(() => {
  class HoistedManagedSocket<TFrameData = Record<string, unknown>> {
    static instances: HoistedManagedSocket[] = []

    readonly options: {
      name: string
      onStatusChange?: (status: string, lastError?: string) => void
      onFrame?: (frame: TFrameData) => void
    }

    start = vi.fn()
    stop = vi.fn()
    refresh = vi.fn()

    constructor(options: HoistedManagedSocket<TFrameData>['options']) {
      this.options = options
      HoistedManagedSocket.instances.push(this)
    }

    emitStatus(status: string, lastError?: string) {
      this.options.onStatusChange?.(status, lastError)
    }

    emitFrame(frame: TFrameData) {
      this.options.onFrame?.(frame)
    }
  }
  return { MockManagedSocket: HoistedManagedSocket }
})

vi.mock('@/lib/ws', () => ({
  ManagedSocket: MockManagedSocket,
}))

describe('socket store', () => {
  beforeEach(() => {
    MockManagedSocket.instances = []
    setActivePinia(createPinia())
  })

  it('starts management sockets, projects statuses, and routes frames to stores', () => {
    const sessionStore = useSessionStore()
    sessionStore.token = 'session-token'
    const systemStore = useSystemStore()
    const tasksStore = useTasksStore()
    const logsStore = useLogsStore()
    const pluginsStore = usePluginsStore()
    const store = useSocketStore()

    store.ensureManagementSockets()

    expect(MockManagedSocket.instances).toHaveLength(4)
    expect(MockManagedSocket.instances[0].start).toHaveBeenCalledTimes(1)
    expect(MockManagedSocket.instances[1].start).toHaveBeenCalledTimes(1)
    expect(MockManagedSocket.instances[2].start).toHaveBeenCalledTimes(1)

    MockManagedSocket.instances[0].emitStatus('authenticated')
    MockManagedSocket.instances[1].emitStatus('reconnecting', 'tasks 连接异常')
    expect(store.snapshots.events.status).toBe('authenticated')
    expect(store.snapshots.tasks.lastError).toBe('tasks 连接异常')

    MockManagedSocket.instances[0].emitFrame({
      timestamp: '2026-04-05T08:00:00Z',
      data: {
        summary: 'adapter ready',
        plugin_id: 'weather',
        registration_state: 'installed',
        desired_state: 'enabled',
        runtime_state: 'running',
        display_state: 'running',
      },
    })
    MockManagedSocket.instances[0].emitFrame({
      timestamp: '2026-04-05T08:00:00Z',
      data: {
        summary: 'plugin state changed',
        plugin_id: 'weather',
        registration_state: 'installed',
        desired_state: 'disabled',
        runtime_state: 'stopping',
        display_state: 'disabling',
      },
    })
    MockManagedSocket.instances[0].emitFrame({
      timestamp: '2026-04-05T08:00:01Z',
      data: {
        summary: 'plugin state changed',
        plugin_id: 'weather',
        registration_state: 'installed',
        desired_state: 'disabled',
        runtime_state: 'stopped',
        display_state: 'disabled',
      },
    })
    MockManagedSocket.instances[1].emitFrame({
      type: 'tasks.updated',
      data: {
        task_id: 'task_1',
        task_type: 'runtime.bootstrap',
        status: 'running',
      },
    })
    MockManagedSocket.instances[2].emitFrame({
      type: 'logs.appended',
      data: {
        log_id: 'log_protocol_live_0001',
        timestamp: '2026-04-05T08:00:01Z',
        level: 'warn',
        protocol: 'onebot11',
        source: 'adapter',
        message: 'log line',
      },
    })
    MockManagedSocket.instances[2].emitFrame({
      type: 'logs.appended',
      data: {
        log_id: 'log_plugin_outbound_0001',
        timestamp: '2026-04-05T08:00:01Z',
        level: 'info',
        protocol: 'onebot11',
        source: 'adapter.onebot11',
        plugin_id: 'weather',
        request_id: 'req_runtime_delivery_0001',
        message: 'plugin weather command echo delivered group message: hello',
      },
    })
    MockManagedSocket.instances[2].emitFrame({
      type: 'logs.appended',
      data: {
        log_id: 'log_runtime_0001',
        timestamp: '2026-04-05T08:00:02Z',
        level: 'info',
        source: 'runtime',
        message: 'runtime line',
      },
    })

    expect(systemStore.recentEvents).toHaveLength(3)
    expect(pluginsStore.items[0].id).toBe('weather')
    expect(pluginsStore.items[0].runtime_state).toBe('stopped')
    expect(pluginsStore.items[0].display_state).toBe('disabled')
    expect(tasksStore.items[0].task_id).toBe('task_1')
    expect(logsStore.items.map((item) => item.message)).toEqual([
      'plugin weather command echo delivered group message: hello',
      'log line',
      'runtime line',
    ])
    expect(pluginsStore.getConsole('weather').filter((item) => item.stream === 'outbound')).toHaveLength(1)
    expect(logsStore.pendingNewCount).toBe(3)
  })

  it('manages the console socket separately and disconnects all sockets', () => {
    const sessionStore = useSessionStore()
    sessionStore.token = 'session-token'
    const pluginsStore = usePluginsStore()
    const store = useSocketStore()

    store.ensureManagementSockets()
    store.setConsolePlugin('weather')

    expect(MockManagedSocket.instances[3].start).toHaveBeenCalledTimes(1)
    expect(MockManagedSocket.instances[3].refresh).toHaveBeenCalledTimes(1)

    MockManagedSocket.instances[3].emitFrame({
      type: 'plugins.console',
      data: {
        plugin_id: 'weather',
        stream: 'stdout',
        text: 'console line',
        timestamp: '2026-04-05T08:00:02Z',
      },
    })
    expect(pluginsStore.getConsole('weather')[0].text).toBe('console line')

    store.disconnectAll()

    expect(MockManagedSocket.instances[0].stop).toHaveBeenCalledTimes(1)
    expect(MockManagedSocket.instances[1].stop).toHaveBeenCalledTimes(1)
    expect(MockManagedSocket.instances[2].stop).toHaveBeenCalledTimes(1)
    expect(MockManagedSocket.instances[3].stop).toHaveBeenCalledTimes(1)
  })
})
