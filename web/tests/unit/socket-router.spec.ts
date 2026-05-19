import { flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { createSocketFrameRouter } from '@/stores/socket-router'

describe('socket frame router', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('debounces service status refreshes while keeping events readable', async () => {
    const dependencies = {
      system: {
        applyEvent: vi.fn(),
        refreshStatus: vi.fn().mockResolvedValue(undefined),
      },
      plugins: {
        upsert: vi.fn(),
      },
      pluginConsole: {
        appendOutboundLog: vi.fn(),
        appendConsole: vi.fn(),
      },
      tasks: {
        upsert: vi.fn(),
      },
      logs: {
        appendBatch: vi.fn(),
      },
      governance: {
        refresh: vi.fn().mockResolvedValue(undefined),
      },
      protocols: {
        applySnapshot: vi.fn(),
      },
    }
    const router = createSocketFrameRouter(dependencies)

    router.handleEventsFrame({
      channel: 'events',
      type: 'events.received',
      timestamp: '2026-04-05T08:00:00Z',
      data: {
        service_status: 'degraded',
        summary: '服务运行条件受限',
        reason: '运行环境尚未准备完成。',
        reason_codes: ['platform.resource_missing'],
      },
    })
    router.handleEventsFrame({
      channel: 'events',
      type: 'events.received',
      timestamp: '2026-04-05T08:00:01Z',
      data: {
        service_status: 'degraded',
        summary: '服务运行条件受限',
        reason: '运行环境尚未准备完成。',
        reason_codes: ['platform.resource_missing'],
      },
    })

    expect(dependencies.system.applyEvent).toHaveBeenCalledTimes(2)
    expect(dependencies.system.refreshStatus).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(120)

    expect(dependencies.system.refreshStatus).toHaveBeenCalledTimes(1)
  })

  it('routes plugin and protocol events to the narrow dependencies', () => {
    const dependencies = {
      system: {
        applyEvent: vi.fn(),
        refreshStatus: vi.fn().mockResolvedValue(undefined),
      },
      plugins: {
        upsert: vi.fn(),
      },
      pluginConsole: {
        appendOutboundLog: vi.fn(),
        appendConsole: vi.fn(),
      },
      tasks: {
        upsert: vi.fn(),
      },
      logs: {
        appendBatch: vi.fn(),
      },
      governance: {
        refresh: vi.fn().mockResolvedValue(undefined),
      },
      protocols: {
        applySnapshot: vi.fn(),
      },
    }
    const router = createSocketFrameRouter(dependencies)

    router.handleEventsFrame({
      channel: 'events',
      type: 'events.received',
      timestamp: '2026-04-05T08:00:00Z',
      data: {
        plugin_id: 'weather',
        registration_state: 'installed',
        desired_state: 'enabled',
        runtime_state: 'running',
        display_state: 'running',
      },
    })
    router.handleEventsFrame({
      channel: 'events',
      type: 'events.received',
      timestamp: '2026-04-05T08:00:02Z',
      data: {
        protocol: 'onebot11',
        protocol_snapshot: {
          protocol: 'onebot11',
          provider: 'standard',
          configured_transports: ['reverse_ws'],
          active_transports: ['reverse_ws'],
          transport_status: [
            {
              transport: 'reverse_ws',
              enabled: true,
              configured: true,
              endpoint: 'ws://127.0.0.1:8080/ws',
              state: 'connected',
              summary: '已连接',
            },
          ],
          readiness_status: 'ready',
          summary: 'OneBot11 已就绪',
          recent_transport_issues: [],
        },
      },
    })

    expect(dependencies.system.applyEvent).toHaveBeenCalledTimes(2)
    expect(dependencies.plugins.upsert).toHaveBeenCalledWith({
      id: 'weather',
      registration_state: 'installed',
      desired_state: 'enabled',
      runtime_state: 'running',
      display_state: 'running',
    })
    expect(dependencies.protocols.applySnapshot).toHaveBeenCalledTimes(1)
  })

  it('routes task, log, and console frames without changing payload semantics', async () => {
    const dependencies = {
      system: {
        applyEvent: vi.fn(),
        refreshStatus: vi.fn().mockResolvedValue(undefined),
      },
      plugins: {
        upsert: vi.fn(),
      },
      pluginConsole: {
        appendOutboundLog: vi.fn(),
        appendConsole: vi.fn(),
      },
      tasks: {
        upsert: vi.fn(),
      },
      logs: {
        appendBatch: vi.fn(),
      },
      governance: {
        refresh: vi.fn().mockResolvedValue(undefined),
      },
      protocols: {
        applySnapshot: vi.fn(),
      },
    }
    const router = createSocketFrameRouter(dependencies)

    router.handleTasksFrame({
      channel: 'tasks',
      type: 'tasks.updated',
      timestamp: '2026-04-05T08:00:03Z',
      data: {
        task_id: 'task_1',
        task_type: 'runtime.bootstrap',
        status: 'running',
        summary: '运行环境准备中',
      },
    })
    router.handleLogsFrame({
      channel: 'logs',
      type: 'logs.appended',
      timestamp: '2026-04-05T08:00:04Z',
      data: {
        log_id: 'log_plugin_outbound_0001',
        timestamp: '2026-04-05T08:00:04Z',
        level: 'info',
        protocol: 'onebot11',
        source: 'adapter.onebot11',
        plugin_id: 'weather',
        request_id: 'req_runtime_delivery_0001',
        message: 'plugin weather command echo delivered group message: hello',
      },
    })
    router.handleConsoleFrame({
      channel: 'plugin_console',
      type: 'plugins.console',
      timestamp: '2026-04-05T08:00:05Z',
      data: {
        plugin_id: 'weather',
        stream: 'stdout',
        text: 'console line',
        timestamp: '2026-04-05T08:00:05Z',
      },
    })

    await flushPromises()

    expect(dependencies.tasks.upsert).toHaveBeenCalledWith({
      task_id: 'task_1',
      task_type: 'runtime.bootstrap',
      status: 'running',
      summary: '运行环境准备中',
    })
    expect(dependencies.logs.appendBatch).toHaveBeenCalledTimes(1)
    expect(dependencies.logs.appendBatch).toHaveBeenCalledWith([
      {
        log_id: 'log_plugin_outbound_0001',
        timestamp: '2026-04-05T08:00:04Z',
        level: 'info',
        protocol: 'onebot11',
        source: 'adapter.onebot11',
        plugin_id: 'weather',
        request_id: 'req_runtime_delivery_0001',
        message: 'plugin weather command echo delivered group message: hello',
      },
    ])
    expect(dependencies.pluginConsole.appendOutboundLog).toHaveBeenCalledTimes(1)
    expect(dependencies.pluginConsole.appendConsole).toHaveBeenCalledWith({
      plugin_id: 'weather',
      stream: 'stdout',
      text: 'console line',
      timestamp: '2026-04-05T08:00:05Z',
    })
  })

  it('debounces governance refresh when governance.changed arrives repeatedly', async () => {
    const dependencies = {
      system: {
        applyEvent: vi.fn(),
        refreshStatus: vi.fn().mockResolvedValue(undefined),
      },
      plugins: {
        upsert: vi.fn(),
      },
      pluginConsole: {
        appendOutboundLog: vi.fn(),
        appendConsole: vi.fn(),
      },
      tasks: {
        upsert: vi.fn(),
      },
      logs: {
        appendBatch: vi.fn(),
      },
      governance: {
        refresh: vi.fn().mockResolvedValue(undefined),
      },
      protocols: {
        applySnapshot: vi.fn(),
      },
    }
    const router = createSocketFrameRouter(dependencies)

    router.handleEventsFrame({
      channel: 'events',
      type: 'events.received',
      timestamp: '2026-04-20T08:00:00Z',
      data: {
        event_type: 'governance.changed',
        summary: '治理设置已更新',
      },
    })
    router.handleEventsFrame({
      channel: 'events',
      type: 'events.received',
      timestamp: '2026-04-20T08:00:01Z',
      data: {
        event_type: 'governance.changed',
        summary: '治理设置已更新',
      },
    })

    expect(dependencies.system.applyEvent).toHaveBeenCalledTimes(2)
    expect(dependencies.governance.refresh).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(120)

    expect(dependencies.governance.refresh).toHaveBeenCalledTimes(1)
  })

  it('runs one more governance refresh when another governance.changed arrives mid-refresh', async () => {
    let resolveRefresh: (() => void) | null = null
    const refreshSpy = vi.fn().mockImplementation(() => new Promise<void>((resolve) => {
      resolveRefresh = resolve
    }))
    const dependencies = {
      system: {
        applyEvent: vi.fn(),
        refreshStatus: vi.fn().mockResolvedValue(undefined),
      },
      plugins: {
        upsert: vi.fn(),
      },
      pluginConsole: {
        appendOutboundLog: vi.fn(),
        appendConsole: vi.fn(),
      },
      tasks: {
        upsert: vi.fn(),
      },
      logs: {
        appendBatch: vi.fn(),
      },
      governance: {
        refresh: refreshSpy,
      },
      protocols: {
        applySnapshot: vi.fn(),
      },
    }
    const router = createSocketFrameRouter(dependencies)

    router.handleEventsFrame({
      channel: 'events',
      type: 'events.received',
      timestamp: '2026-04-20T08:00:00Z',
      data: {
        event_type: 'governance.changed',
        summary: '治理设置已更新',
      },
    })
    await vi.advanceTimersByTimeAsync(120)
    expect(refreshSpy).toHaveBeenCalledTimes(1)

    router.handleEventsFrame({
      channel: 'events',
      type: 'events.received',
      timestamp: '2026-04-20T08:00:01Z',
      data: {
        event_type: 'governance.changed',
        summary: '治理设置已更新',
      },
    })

    expect(refreshSpy).toHaveBeenCalledTimes(1)

    resolveRefresh?.()
    await flushPromises()
    await vi.advanceTimersByTimeAsync(120)

    expect(refreshSpy).toHaveBeenCalledTimes(2)
  })

  it('batches multiple log frames into a single appendBatch call', async () => {
    const dependencies = {
      system: {
        applyEvent: vi.fn(),
        refreshStatus: vi.fn().mockResolvedValue(undefined),
      },
      plugins: {
        upsert: vi.fn(),
      },
      pluginConsole: {
        appendOutboundLog: vi.fn(),
        appendConsole: vi.fn(),
      },
      tasks: {
        upsert: vi.fn(),
      },
      logs: {
        appendBatch: vi.fn(),
      },
      governance: {
        refresh: vi.fn().mockResolvedValue(undefined),
      },
      protocols: {
        applySnapshot: vi.fn(),
      },
    }
    const router = createSocketFrameRouter(dependencies)

    router.handleLogsFrame({
      channel: 'logs',
      type: 'logs.appended',
      timestamp: '2026-04-05T08:00:01Z',
      data: {
        log_id: 'log_1',
        timestamp: '2026-04-05T08:00:01Z',
        level: 'info',
        source: 'runtime',
        message: 'first',
      },
    })
    router.handleLogsFrame({
      channel: 'logs',
      type: 'logs.appended',
      timestamp: '2026-04-05T08:00:02Z',
      data: {
        log_id: 'log_2',
        timestamp: '2026-04-05T08:00:02Z',
        level: 'info',
        source: 'runtime',
        message: 'second',
      },
    })

    expect(dependencies.logs.appendBatch).not.toHaveBeenCalled()

    await flushPromises()

    expect(dependencies.logs.appendBatch).toHaveBeenCalledTimes(1)
    expect(dependencies.logs.appendBatch).toHaveBeenCalledWith([
      {
        log_id: 'log_1',
        timestamp: '2026-04-05T08:00:01Z',
        level: 'info',
        source: 'runtime',
        message: 'first',
      },
      {
        log_id: 'log_2',
        timestamp: '2026-04-05T08:00:02Z',
        level: 'info',
        source: 'runtime',
        message: 'second',
      },
    ])
    expect(dependencies.pluginConsole.appendOutboundLog).toHaveBeenCalledTimes(2)
  })
})
