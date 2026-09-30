import { createPinia, getActivePinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import DashboardPage from '@/views/dashboard/DashboardView.vue'
import { useAdaptersStore } from '@/stores/adapters'
import { useSystemStore } from '@/stores/system'

const feedbackMock = vi.hoisted(() => ({
  notifyError: vi.fn(),
  notifySuccess: vi.fn(),
  useToastFeedback: vi.fn(),
}))

vi.mock('@/adapter/feedback', () => ({
  notifyError: feedbackMock.notifyError,
  notifySuccess: feedbackMock.notifySuccess,
  useToastFeedback: feedbackMock.useToastFeedback,
}))

function toastMessages() {
  return feedbackMock.useToastFeedback.mock.calls
    .map(([source]) => {
      if (typeof source === 'function') {
        return source()?.message
      }
      return source.value?.message
    })
    .filter((message): message is string => Boolean(message))
}

function createAdapterSnapshots(overrides: Record<string, unknown> = {}) {
  return [{ id: 'onebot11', protocol: 'onebot11', display_name: 'OneBot11', enabled: true, state: 'connected', summary: '已连接', onebot11: {
    protocol: 'onebot11',
    provider: 'napcat',
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
  } }]
}

function mockDashboardRefreshes() {
  const systemStore = useSystemStore()
  const adaptersStore = useAdaptersStore()
  vi.spyOn(systemStore, 'refreshAll').mockResolvedValue(undefined)
  vi.spyOn(adaptersStore, 'refresh').mockImplementation(async () => ({ adapters: adaptersStore.adapters, available_protocols: [] }))
  return { adaptersStore, systemStore }
}

function createDashboardRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'status', component: DashboardPage },
      { path: '/protocols', name: 'protocols', component: { template: '<div>protocols</div>' } },
      { path: '/logs', name: 'logs', component: { template: '<div>logs</div>' } },
      { path: '/logs/history', name: 'logs-history', component: { template: '<div>history</div>' } },
      { path: '/plugins', name: 'plugins', component: { template: '<div>plugins</div>' } },
      { path: '/plugins/:id', name: 'plugin-detail', component: { template: '<div>plugin</div>' } },
    ],
  })
}

async function mountDashboard() {
  const router = createDashboardRouter()
  await router.push('/')
  await router.isReady()
  const wrapper = mount(DashboardPage, { global: { plugins: [getActivePinia()!, router] } })
  await flushPromises()
  return { router, wrapper }
}

describe('DashboardPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    feedbackMock.notifyError.mockClear()
    feedbackMock.notifySuccess.mockClear()
    feedbackMock.useToastFeedback.mockClear()
  })

  it('leads with the pending issue, lists every readiness check and prepares the missing runtime', async () => {
    const { systemStore: store } = mockDashboardRefreshes()
    store.readiness = {
      status: 'degraded',
      checks: { config: 'ok', render: 'resource_missing' },
      issues: [{ code: 'platform.resource_missing', runtime_resources: ['chromium'], severity: 'warning', summary: '浏览器运行资源缺失', remediation: '准备运行环境后重试。' }],
    }
    const prepare = vi.spyOn(store, 'bootstrapManagedRuntime').mockResolvedValue({ task_id: 'fixture-runtime-task' })
    const { wrapper } = await mountDashboard()

    expect(wrapper.get('[data-testid="dashboard-attention"]').text()).toContain('1 项需要处理：浏览器运行资源缺失')
    expect(wrapper.get('[data-testid="dashboard-attention"]').text()).toContain('其余 1 项就绪检查通过。')
    const checks = wrapper.get('[data-testid="dashboard-checks"]')
    expect(checks.text()).toContain('图片生成')
    expect(checks.text()).toContain('缺少运行资源')
    expect(checks.get('details.status-technical').attributes('open')).toBeUndefined()

    await wrapper.get('[data-testid="readiness-prepare-runtime"]').trigger('click')
    await flushPromises()
    expect(prepare).toHaveBeenCalledWith(['chromium'])

    store.readiness.issues = [{ code: 'database.unavailable', severity: 'error', summary: '数据库不可用' }]
    await flushPromises()
    expect(wrapper.find('[data-testid="readiness-prepare-runtime"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="dashboard-attention"]').attributes('data-tone')).toBe('danger')
    wrapper.unmount()
  })

  it('shows the header actions, the connections and the runtime facts without any plugin content', async () => {
    const { adaptersStore, systemStore: store } = mockDashboardRefreshes()
    store.health = { status: 'ok' }
    store.readiness = { status: 'ready' }
    store.system = {
      status: 'running',
      adapters: [{ id: 'onebot11', protocol: 'onebot11', enabled: true, state: 'connected' }],
      active_plugins: 2,
      running_plugins: 1,
      failed_plugins: 1,
      db_schema_version: '000001',
      uptime_seconds: 120,
    }
    adaptersStore.adapters = createAdapterSnapshots()
    const createBackupSpy = vi.spyOn(store as never, 'createBackup').mockResolvedValue({ task_id: 'task_backup_create_0001' })
    const exportDiagnosticsSpy = vi.spyOn(store as never, 'exportDiagnostics').mockResolvedValue(undefined)

    const { wrapper } = await mountDashboard()

    expect(store.refreshAll).toHaveBeenCalledTimes(1)
    expect(adaptersStore.refresh).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-testid="dashboard-attention"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('运行中 · 已运行 2 分钟 0 秒')
    expect(wrapper.get('[data-testid="dashboard-connections"]').text()).toContain('OneBot11')
    expect(wrapper.get('[data-testid="dashboard-connections"]').text()).toContain('1 / 1 已连接')
    expect(wrapper.get('[data-testid="dashboard-runtime-info"]').text()).toContain('schema 000001')
    expect(wrapper.text()).not.toContain('插件')

    await wrapper.findAll('button').find(candidate => candidate.text().includes('创建备份'))!.trigger('click')
    await wrapper.findAll('button').find(candidate => candidate.text().includes('导出诊断包'))!.trigger('click')
    expect(createBackupSpy).toHaveBeenCalledTimes(1)
    expect(exportDiagnosticsSpy).toHaveBeenCalledTimes(1)
  })

  it('updates the displayed uptime every second without refreshing dashboard data', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-12T00:00:00Z'))
    let wrapper: { text: () => string; unmount: () => void } | null = null

    try {
      const { adaptersStore, systemStore: store } = mockDashboardRefreshes()
      store.health = { status: 'ok' }
      store.readiness = { status: 'ready' }
      store.system = {
        status: 'running',
        adapters: [{ id: 'onebot11', protocol: 'onebot11', enabled: true, state: 'connected' }],
        active_plugins: 2,
        uptime_seconds: 120,
      }
      adaptersStore.adapters = createAdapterSnapshots()

      wrapper = (await mountDashboard()).wrapper

      expect(wrapper.text()).toContain('2 分钟 0 秒')
      expect(store.refreshAll).toHaveBeenCalledTimes(1)

      await vi.advanceTimersByTimeAsync(6000)
      await flushPromises()

      expect(wrapper.text()).toContain('2 分钟 6 秒')
      expect(store.refreshAll).toHaveBeenCalledTimes(1)
    } finally {
      wrapper?.unmount()
      vi.useRealTimers()
    }
  })

  it('lists system changes, leaves plugin lifecycle events out and links connection changes to the protocols page', async () => {
    const { adaptersStore, systemStore: store } = mockDashboardRefreshes()
    store.readiness = { status: 'ready' }
    adaptersStore.adapters = createAdapterSnapshots()
    store.recentEvents = [
      { timestamp: '2026-06-12T00:01:00Z', summary: '插件 weather 运行中', payload: { plugin_id: 'weather', state: 'running', commands: [], command_conflicts: [] } },
      { timestamp: '2026-06-12T00:00:00Z', summary: '协议连接正常', payload: { connection_status: 'connected', summary: '协议连接正常' } },
    ]

    const { router, wrapper } = await mountDashboard()
    const events = wrapper.get('[data-testid="dashboard-events"]')
    expect(events.text()).toContain('协议连接正常')
    expect(events.text()).not.toContain('插件 weather 运行中')

    await events.get('.status-event__summary--link').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.name).toBe('protocols')
  })

  it('shows a protocol reminder when the protocol snapshot is degraded with transport issues', async () => {
    const { adaptersStore, systemStore: store } = mockDashboardRefreshes()
    store.health = { status: 'ok' }
    store.readiness = { status: 'degraded' }
    store.system = {
      status: 'running',
      adapters: [{ id: 'onebot11', protocol: 'onebot11', enabled: true, state: 'reconnecting' }],
      active_plugins: 2,
      uptime_seconds: 120,
    }
    adaptersStore.adapters = createAdapterSnapshots({
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

    const { wrapper } = await mountDashboard()

    expect(wrapper.text()).not.toContain('OneBot 主动连接已断开，正在重试。')
    expect(wrapper.text()).not.toContain('adapter.transport_forward_ws_session_lost')
    expect(feedbackMock.useToastFeedback).toHaveBeenCalledTimes(3)
    const protocolToastSource = feedbackMock.useToastFeedback.mock.calls[2][0] as { value: { message?: string | null } | null }
    expect(protocolToastSource.value?.message).toBe('协议提醒：OneBot11：OneBot 主动连接已断开，正在重试。')
  })

  it('renders readiness issues from the readiness snapshot', async () => {
    const { adaptersStore, systemStore: store } = mockDashboardRefreshes()
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
      adapters: [{ id: 'onebot11', protocol: 'onebot11', enabled: true, state: 'auth_failed' }],
      active_plugins: 2,
      uptime_seconds: 120,
    }
    adaptersStore.adapters = createAdapterSnapshots()

    const { wrapper } = await mountDashboard()

    expect(wrapper.text()).toContain('就绪检查')
    expect(toastMessages()).toContain('协议连接警告：OneBot authentication failed')
    expect(wrapper.text()).not.toContain('运行条件受限')
    expect(wrapper.findAll('[data-tone="success"]').length).toBeGreaterThan(0)
    expect(wrapper.text()).toContain('adapter.auth_failed')
    expect(wrapper.text()).toContain('请检查对应连接方式的访问令牌后重试连接。')
  })

  it('shows readiness issues and their recovery guidance', async () => {
    const { adaptersStore, systemStore: store } = mockDashboardRefreshes()
    store.health = { status: 'ok' }
    store.readiness = {
      status: 'degraded',
      reason: '运行条件未满足',
      reason_codes: ['platform.resource_missing'],
      issues: [
        {
          code: 'platform.resource_missing',
          runtime_resources: ['chromium'],
          severity: 'warning',
          summary: '图片渲染 Chromium 尚未准备完成。',
          remediation: '请先准备图片渲染 Chromium。',
        },
      ],
    }
    store.system = {
      status: 'running',
      adapters: [{ id: 'onebot11', protocol: 'onebot11', enabled: true, state: 'idle' }],
      active_plugins: 0,
      uptime_seconds: 17,
    }
    adaptersStore.adapters = createAdapterSnapshots()

    const { wrapper } = await mountDashboard()

    expect(wrapper.text()).toContain('运行条件受限')
    expect(wrapper.get('[data-testid="dashboard-attention"]').text()).toContain('1 项需要处理：图片渲染 Chromium 尚未准备完成。')
    expect(wrapper.get('[data-testid="dashboard-attention"]').text()).toContain('请先准备图片渲染 Chromium。')
    expect(toastMessages()).toContain('运行条件受限：图片渲染 Chromium 尚未准备完成。')
  })

  it('deduplicates readiness issue codes already represented by issue rows', async () => {
    const { adaptersStore, systemStore: store } = mockDashboardRefreshes()
    store.health = { status: 'ok' }
    store.readiness = {
      status: 'degraded',
      reason: 'Render resources are incomplete',
      reason_codes: ['platform.resource_missing', 'platform.resource_missing'],
      issues: [
        {
          code: 'platform.resource_missing',
          runtime_resources: ['chromium'],
          severity: 'warning',
          summary: '图片渲染 Chromium 尚未准备完成',
          remediation: '请先准备图片渲染 Chromium，或在配置中显式设置浏览器路径。',
        },
        {
          code: 'platform.resource_missing',
          runtime_resources: ['chromium'],
          severity: 'warning',
          summary: '图片渲染 Chromium 尚未准备完成',
          remediation: '请先准备图片渲染 Chromium，或在配置中显式设置浏览器路径。',
        },
      ],
    }
    store.system = {
      status: 'running',
      adapters: [{ id: 'onebot11', protocol: 'onebot11', enabled: true, state: 'idle' }],
      active_plugins: 0,
      uptime_seconds: 50,
    }
    adaptersStore.adapters = createAdapterSnapshots()

    const { wrapper } = await mountDashboard()

    expect(wrapper.findAll('[data-testid="dashboard-checks"] .status-issue')).toHaveLength(1)
    expect(wrapper.get('[data-testid="dashboard-attention"]').text()).toContain('1 项需要处理')
    expect((wrapper.text().match(/platform\.resource_missing/g) ?? []).length).toBe(1)
  })
})
