import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import DashboardPage from '@/views/dashboard/DashboardView.vue'
import { useProtocolsStore } from '@/stores/protocols'
import { useSystemStore } from '@/stores/system'
import { useTasksStore } from '@/stores/tasks'

function createProtocolSnapshot(overrides: Record<string, unknown> = {}) {
  return {
    protocol: 'onebot11',
    configured_transports: ['forward_ws'],
    active_transports: ['forward_ws'],
    transport_status: [
      { transport: 'reverse_ws', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
      { transport: 'forward_ws', enabled: true, configured: true, endpoint: 'ws://127.0.0.1:8089', state: 'connected', summary: '主动连接已建立' },
      { transport: 'http_api', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
      { transport: 'webhook', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
    ],
    readiness_status: 'ready',
    summary: 'OneBot11 主动连接已就绪',
    recent_transport_issues: [],
    ...overrides,
  }
}

function mockDashboardRefreshes() {
  const systemStore = useSystemStore()
  const protocolsStore = useProtocolsStore()
  const tasksStore = useTasksStore()
  vi.spyOn(systemStore, 'refresh').mockResolvedValue(undefined)
  vi.spyOn(protocolsStore, 'refresh').mockImplementation(async () => ({ snapshot: protocolsStore.snapshot }))
  return { protocolsStore, systemStore, tasksStore }
}

function createDashboardRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'status', component: DashboardPage },
      { path: '/tasks', name: 'tasks', component: { template: '<div>tasks</div>' } },
      { path: '/protocols', name: 'protocols', component: { template: '<div>protocols</div>' } },
      { path: '/logs', name: 'logs', component: { template: '<div>logs</div>' } },
      { path: '/plugins/:id', name: 'plugin-detail', component: { template: '<div>plugin</div>' } },
      { path: '/render/templates/:templateId?', name: 'render-templates', component: { template: '<div>template</div>' } },
    ],
  })
}

describe('DashboardPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders a compact status page with overview cards, tabs, and bottom workbench cards', async () => {
    const router = createDashboardRouter()
    await router.push('/')
    await router.isReady()

    const { protocolsStore, systemStore: store } = mockDashboardRefreshes()
    store.health = { status: 'ok' }
    store.readiness = { status: 'ready' }
    store.system = {
      status: 'running',
      adapter_state: 'connected',
      active_plugins: 2,
      uptime_seconds: 120,
    }
    protocolsStore.snapshot = createProtocolSnapshot()

    const createBackupSpy = vi.spyOn(store as never, 'createBackup').mockResolvedValue({ task_id: 'task_backup_create_0001' })
    const exportDiagnosticsSpy = vi.spyOn(store as never, 'exportDiagnostics').mockResolvedValue(undefined)

    const wrapper = mount(DashboardPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    const backupButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('创建备份'))
    const diagnosticsButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('导出诊断包'))

    expect(backupButton).toBeTruthy()
    expect(diagnosticsButton).toBeTruthy()
    expect(wrapper.text()).toContain('系统工具')
    expect(wrapper.text()).toContain('连接状态')
    expect(wrapper.text()).toContain('近期变化')
    expect(wrapper.text()).toContain('就绪检查')
    expect(wrapper.find('[data-testid="dashboard-connection-card"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="dashboard-overview-grid"]').exists()).toBe(true)
    expect(wrapper.findAll('.dashboard-overview-grid .stat-card')).toHaveLength(4)
    expect(wrapper.find('.dashboard-main-grid .ant-tabs').exists()).toBe(true)
    expect(wrapper.findAll('.dashboard-bottom-grid > .ant-card')).toHaveLength(3)
    expect(wrapper.find('[data-testid="dashboard-protocol-alert"]').exists()).toBe(false)
    expect(wrapper.find('.dashboard-hero-card').exists()).toBe(false)
    expect(wrapper.find('.status-badge').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('聚合 health、ready、system status')

    await backupButton!.trigger('click')
    await diagnosticsButton!.trigger('click')

    expect(createBackupSpy).toHaveBeenCalledTimes(1)
    expect(exportDiagnosticsSpy).toHaveBeenCalledTimes(1)
  })

  it('submits render preview requests from the tools section', async () => {
    const router = createDashboardRouter()
    await router.push('/')
    await router.isReady()

    const { protocolsStore, systemStore: store } = mockDashboardRefreshes()
    store.health = { status: 'ok' }
    store.readiness = { status: 'ready' }
    store.system = {
      status: 'running',
      adapter_state: 'connected',
      active_plugins: 2,
      uptime_seconds: 120,
    }
    protocolsStore.snapshot = createProtocolSnapshot()

    const previewSpy = vi.spyOn(store as never, 'previewRender').mockResolvedValue({ task_id: 'task_render_preview_0001' })

    const wrapper = mount(DashboardPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    const previewButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('图片预览'))
    expect(previewButton).toBeTruthy()

    await previewButton!.trigger('click')
    await flushPromises()

    const templateInput = wrapper.find('input[placeholder="help.menu"]')
    await templateInput.setValue('help.menu')
    const submitButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('生成预览'))
    expect(submitButton).toBeTruthy()
    await submitButton!.trigger('click')

    expect(previewSpy).toHaveBeenCalledTimes(1)
  })

  it('shows a protocol reminder when the protocol snapshot is degraded with transport issues', async () => {
    const router = createDashboardRouter()
    await router.push('/')
    await router.isReady()

    const { protocolsStore, systemStore: store } = mockDashboardRefreshes()
    store.health = { status: 'ok' }
    store.readiness = { status: 'degraded' }
    store.system = {
      status: 'running',
      adapter_state: 'reconnecting',
      active_plugins: 2,
      uptime_seconds: 120,
    }
    protocolsStore.snapshot = createProtocolSnapshot({
      readiness_status: 'degraded',
      summary: 'OneBot11 传输链路部分可用',
      recent_transport_issues: [
        {
          code: 'adapter.transport_forward_ws_session_lost',
          severity: 'warning',
          summary: 'OneBot 主动连接已断开，正在重试。',
        },
      ],
    })

    const wrapper = mount(DashboardPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.find('[data-testid="dashboard-protocol-alert"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('协议提醒')
    expect(wrapper.text()).toContain('OneBot 主动连接已断开，正在重试。')
    expect(wrapper.text()).toContain('adapter.transport_forward_ws_session_lost')
    expect(wrapper.text()).toContain('查看协议中心')
    expect(wrapper.text()).toContain('查看实时日志')

    const realtimeLogsButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('查看实时日志'))
    expect(realtimeLogsButton).toBeTruthy()
    await realtimeLogsButton!.trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.name).toBe('logs')
    expect(router.currentRoute.value.query.protocol).toBe('onebot11')
  })

  it('renders readiness issues instead of legacy checks', async () => {
    const router = createDashboardRouter()
    await router.push('/')
    await router.isReady()

    const { protocolsStore, systemStore: store } = mockDashboardRefreshes()
    store.health = { status: 'ok' }
    store.readiness = {
      status: 'ready',
      issues: [
        {
          code: 'adapter.auth_failed',
          severity: 'warning',
          summary: 'OneBot authentication failed',
          remediation: '请检查对应连接方式的访问令牌后重试连接。',
        },
      ],
    }
    store.system = {
      status: 'running',
      adapter_state: 'auth_failed',
      active_plugins: 2,
      uptime_seconds: 120,
    }
    protocolsStore.snapshot = createProtocolSnapshot()

    const wrapper = mount(DashboardPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('就绪检查')
    expect(wrapper.text()).toContain('协议连接警告')
    expect(wrapper.text()).not.toContain('运行条件受限')
    expect(wrapper.findAll('.stat-card--success').length).toBeGreaterThan(0)
    expect(wrapper.text()).toContain('adapter.auth_failed')
    expect(wrapper.text()).toContain('请检查对应连接方式的访问令牌后重试连接。')
    expect(wrapper.text()).not.toContain('config = ok')
  })

  it('shows degraded readiness without the old explanatory note', async () => {
    const router = createDashboardRouter()
    await router.push('/')
    await router.isReady()

    const { protocolsStore, systemStore: store } = mockDashboardRefreshes()
    store.health = { status: 'ok' }
    store.readiness = {
      status: 'degraded',
      reason: '运行条件未满足',
      reason_codes: ['platform.resource_missing'],
      issues: [
        {
          code: 'platform.resource_missing',
          severity: 'warning',
          summary: 'Python 运行环境尚未准备完成。',
          remediation: '请先准备 Python 运行环境。',
        },
      ],
    }
    store.system = {
      status: 'running',
      adapter_state: 'idle',
      active_plugins: 0,
      uptime_seconds: 17,
    }
    protocolsStore.snapshot = createProtocolSnapshot()

    const wrapper = mount(DashboardPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('运行条件受限')
    expect(wrapper.text()).toContain('管理面可用，但依赖 Python 运行环境的功能暂不可用。')
    expect(wrapper.text()).toContain('管理面可用')
    expect(wrapper.text()).not.toContain('degraded')
    expect(wrapper.text()).not.toContain('性能降级')
    expect(wrapper.text()).not.toContain('健康检查正常，说明管理面可用；就绪状态受限，说明仍有运行条件未满足。')
  })

  it('deduplicates readiness issue codes already represented by issue cards', async () => {
    const router = createDashboardRouter()
    await router.push('/')
    await router.isReady()

    const { protocolsStore, systemStore: store } = mockDashboardRefreshes()
    store.health = { status: 'ok' }
    store.readiness = {
      status: 'degraded',
      reason: 'Render resources are incomplete',
      reason_codes: ['platform.resource_missing', 'platform.resource_missing'],
      issues: [
        {
          code: 'platform.resource_missing',
          severity: 'warning',
          summary: 'Chromium 资源尚未准备完成',
          remediation: '请先准备 Chromium 浏览环境，或在配置中显式设置浏览器路径。',
        },
        {
          code: 'platform.resource_missing',
          severity: 'warning',
          summary: 'Chromium 资源尚未准备完成',
          remediation: '请先准备 Chromium 浏览环境，或在配置中显式设置浏览器路径。',
        },
      ],
    }
    store.system = {
      status: 'running',
      adapter_state: 'idle',
      active_plugins: 0,
      uptime_seconds: 50,
    }
    protocolsStore.snapshot = createProtocolSnapshot()

    const wrapper = mount(DashboardPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.findAll('.issues-list .issue-alert-card')).toHaveLength(1)
    expect(wrapper.text()).toContain('platform.resource_missing')
    expect((wrapper.text().match(/platform\.resource_missing/g) ?? []).length).toBe(1)
    expect(wrapper.text()).not.toContain('原因代码')
  })

  it('renders recovery summary as a dedicated dashboard block', async () => {
    const router = createDashboardRouter()
    await router.push('/')
    await router.isReady()

    const { protocolsStore, systemStore: store } = mockDashboardRefreshes()
    store.health = { status: 'ok' }
    store.readiness = { status: 'degraded' }
    store.system = {
      status: 'running',
      adapter_state: 'connected',
      active_plugins: 2,
      uptime_seconds: 120,
      recovery_summary: {
        status: 'degraded',
        phase: 'post_startup',
        operation: 'upgrade',
        created_at: '2026-04-02T08:00:00Z',
        updated_at: '2026-04-02T08:01:00Z',
        source_core_version: '0.1.0',
        target_core_version: '0.2.0',
        source_config_schema_version: '2',
        target_config_schema_version: '2',
        source_db_schema_version: '0014',
        target_db_schema_version: '0014',
        issues: [
          {
            code: 'recovery.plugin_min_core_version',
            severity: 'warning',
            summary: '插件 weather-pro 需要更高版本的 RayleaBot core。',
            remediation: '升级程序或安装与当前版本兼容的插件包后，再手动重新启用该插件。',
          },
        ],
        skipped_plugins: [
          {
            plugin_id: 'weather-pro',
            version: '1.4.0',
            reason_code: 'plugin.min_core_version',
            summary: '插件最低 core 版本要求不满足，已保留安装目录并跳过自动启用。',
            review_id: 'review_weather_pro',
            review_status: 'pending',
            manual_action: '升级程序或重新安装兼容版本插件。',
          },
          {
            plugin_id: 'legacy-plugin',
            version: '1.0.0',
            reason_code: 'plugin.platform_mismatch',
            summary: '插件平台兼容性不满足，已保留安装目录并跳过自动启用。',
            review_id: 'review_legacy_plugin',
            review_status: 'confirmed',
            reviewed_at: '2026-04-02T08:03:00Z',
            reviewed_by: 'alice',
            manual_action: '安装适用于当前平台的插件包。',
          },
        ],
        manual_actions: ['处理被跳过插件的兼容性问题后，再在管理面中手动重新启用。'],
        next_steps: ['查看恢复摘要中的跳过插件列表并完成兼容性处理。', '通过管理面、Launcher 或 diagnostics 复核 recovery_summary。'],
        audit: [
          {
            task_id: 'task_recovery_confirm_0001',
            created_at: '2026-04-02T08:02:00Z',
            operator_id: 'alice',
            note: '已确认当前跳过状态。',
            items: [
              {
                review_id: 'review_archived',
                plugin_id: 'archived-plugin',
                reason_code: 'plugin.platform_mismatch',
                summary: '插件平台兼容性不满足，已保留安装目录并跳过自动启用。',
                version: '1.0.0',
              },
            ],
          },
          {
            task_id: 'task_recovery_confirm_0002',
            created_at: '2026-04-02T08:05:00Z',
            operator_id: 'bob',
            note: '已记录平台兼容性处理结论。',
            items: [
              {
                review_id: 'review_legacy_plugin',
                plugin_id: 'legacy-plugin',
                reason_code: 'plugin.platform_mismatch',
                summary: '插件平台兼容性不满足，已保留安装目录并跳过自动启用。',
                version: '1.0.0',
              },
            ],
          },
        ],
      },
    }
    protocolsStore.snapshot = createProtocolSnapshot()

    const wrapper = mount(DashboardPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('恢复兼容性')
    expect(wrapper.text()).toContain('需要人工处理')
    expect(wrapper.text()).toContain('weather-pro')
    expect(wrapper.text()).toContain('待确认')
    expect(wrapper.text()).toContain('处理被跳过插件的兼容性问题后，再在管理面中手动重新启用。')
    expect(wrapper.text()).toContain('查看恢复摘要中的跳过插件列表并完成兼容性处理。')
    expect(wrapper.text()).toContain('通过管理面、Launcher 或 diagnostics 复核 recovery_summary。')
    expect(wrapper.text()).toContain('最近确认记录')
    expect(wrapper.text()).toContain('alice')
    expect(wrapper.text()).toContain('已确认当前跳过状态。')
    expect(wrapper.find('[data-testid="recovery-plugin-card-review_weather_pro"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="recovery-plugin-card-review_legacy_plugin"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-testid="recovery-audit-entry"]')).toHaveLength(2)
    expect(wrapper.findAll('[data-testid="recovery-manual-action"]')).toHaveLength(1)
    expect(wrapper.findAll('[data-testid="recovery-next-step"]')).toHaveLength(2)

    await wrapper.find('[data-testid="recovery-filter-confirmed"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="recovery-plugin-card-review_weather_pro"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="recovery-plugin-card-review_legacy_plugin"]').exists()).toBe(true)

    await wrapper.find('[data-testid="recovery-filter-all"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="recovery-plugin-card-review_weather_pro"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="recovery-plugin-card-review_legacy_plugin"]').exists()).toBe(true)
  })

  it('submits recovery confirm, recovery recheck, and runtime bootstrap tasks from the recovery block', async () => {
    const router = createDashboardRouter()
    await router.push('/')
    await router.isReady()

    const { protocolsStore, systemStore: store, tasksStore } = mockDashboardRefreshes()
    store.health = { status: 'ok' }
    store.readiness = { status: 'degraded' }
    store.system = {
      status: 'running',
      adapter_state: 'connected',
      active_plugins: 2,
      uptime_seconds: 120,
      recovery_summary: {
        status: 'degraded',
        phase: 'post_startup',
        operation: 'upgrade',
        created_at: '2026-04-02T08:00:00Z',
        updated_at: '2026-04-02T08:01:00Z',
        issues: [
          {
            code: 'platform.resource_missing',
            severity: 'warning',
            summary: 'Chromium 资源尚未准备完成。',
            remediation: '请先准备 Chromium 浏览环境。',
          },
        ],
        skipped_plugins: [
          {
            plugin_id: 'weather-pro',
            reason_code: 'plugin.min_core_version',
            summary: '插件最低 core 版本要求不满足。',
            review_id: 'review_weather_pro',
            review_status: 'pending',
            manual_action: '升级程序或重新安装兼容版本插件。',
          },
        ],
        manual_actions: ['升级程序或重新安装兼容版本插件。'],
        next_steps: ['通过管理面、Launcher 或 diagnostics 复核 recovery_summary。'],
      },
    }
    protocolsStore.snapshot = createProtocolSnapshot()
    vi.spyOn(tasksStore, 'findInProgressTaskByType').mockResolvedValue(null)
    const confirmSpy = vi.spyOn(store as never, 'confirmRecovery').mockResolvedValue({ task_id: 'task_recovery_confirm_0001' })
    const recheckSpy = vi.spyOn(store as never, 'recheckRecovery').mockResolvedValue({ task_id: 'task_recovery_recheck_0001' })
    const bootstrapSpy = vi.spyOn(store as never, 'bootstrapManagedRuntime').mockResolvedValue({ task_id: 'task_runtime_bootstrap_0001' })

    const wrapper = mount(DashboardPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    const recheckButton = wrapper.find('[data-testid="recovery-recheck-button"]')
    const confirmButton = wrapper.find('[data-testid="recovery-confirm-button"]')
    const bootstrapButton = wrapper.find('[data-testid="runtime-bootstrap-button"]')
    const pluginLink = wrapper.find('[data-testid="recovery-plugin-link-weather-pro"]')
    const checkbox = wrapper.find('[data-testid="recovery-confirm-checkbox-review_weather_pro"] input[type="checkbox"]')
    const noteInput = wrapper.find('textarea')

    expect(recheckButton.exists()).toBe(true)
    expect(confirmButton.exists()).toBe(true)
    expect(bootstrapButton.exists()).toBe(true)
    expect(pluginLink.exists()).toBe(true)
    expect(checkbox.exists()).toBe(true)
    expect(noteInput.exists()).toBe(true)

    await pluginLink.trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.name).toBe('plugin-detail')
    expect(router.currentRoute.value.params.id).toBe('weather-pro')

    await router.push('/')
    await flushPromises()

    await checkbox.setValue(true)
    await noteInput.setValue('已确认当前跳过状态。')
    await confirmButton.trigger('click')
    await flushPromises()
    expect(confirmSpy).toHaveBeenCalledWith({
      review_ids: ['review_weather_pro'],
      note: '已确认当前跳过状态。',
    })
    expect(router.currentRoute.value.name).toBe('tasks')
    expect(router.currentRoute.value.query.task_id).toBe('task_recovery_confirm_0001')

    await router.push('/')
    await flushPromises()

    await recheckButton.trigger('click')
    await flushPromises()
    expect(recheckSpy).toHaveBeenCalledTimes(1)
    expect(router.currentRoute.value.name).toBe('tasks')
    expect(router.currentRoute.value.query.task_id).toBe('task_recovery_recheck_0001')

    await router.push('/')
    await flushPromises()

    await bootstrapButton.trigger('click')
    await flushPromises()
    expect(bootstrapSpy).toHaveBeenCalledWith(['chromium'])
    expect(router.currentRoute.value.name).toBe('tasks')
    expect(router.currentRoute.value.query.task_id).toBe('task_runtime_bootstrap_0001')
  })

  it('opens the existing task instead of submitting duplicate recovery work', async () => {
    const router = createDashboardRouter()
    await router.push('/')
    await router.isReady()

    const { protocolsStore, systemStore: store, tasksStore } = mockDashboardRefreshes()
    store.health = { status: 'ok' }
    store.readiness = { status: 'degraded' }
    store.system = {
      status: 'running',
      adapter_state: 'connected',
      active_plugins: 2,
      uptime_seconds: 120,
      recovery_summary: {
        status: 'degraded',
        phase: 'post_startup',
        operation: 'upgrade',
        created_at: '2026-04-02T08:00:00Z',
        updated_at: '2026-04-02T08:01:00Z',
        issues: [
          {
            code: 'platform.resource_missing',
            severity: 'warning',
            summary: 'Chromium 资源尚未准备完成。',
            remediation: '请先准备 Chromium 浏览环境。',
          },
        ],
        skipped_plugins: [],
        manual_actions: [],
        next_steps: [],
      },
    }
    protocolsStore.snapshot = createProtocolSnapshot()

    const findTaskSpy = vi
      .spyOn(tasksStore, 'findInProgressTaskByType')
      .mockImplementation(async (taskType: string) =>
        taskType === 'recovery.recheck' ? { task_id: 'task_recovery_recheck_existing' } : null,
      )
    const recheckSpy = vi.spyOn(store as never, 'recheckRecovery').mockResolvedValue({ task_id: 'task_recovery_recheck_0001' })

    const wrapper = mount(DashboardPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    const recheckButton = wrapper.find('[data-testid="recovery-recheck-button"]')
    expect(recheckButton.exists()).toBe(true)

    await recheckButton.trigger('click')
    await flushPromises()

    expect(findTaskSpy).toHaveBeenCalledWith('recovery.recheck')
    expect(recheckSpy).not.toHaveBeenCalled()
    expect(router.currentRoute.value.name).toBe('tasks')
    expect(router.currentRoute.value.query.task_id).toBe('task_recovery_recheck_existing')
  })
})
