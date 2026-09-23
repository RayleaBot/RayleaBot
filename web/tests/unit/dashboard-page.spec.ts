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
      { path: '/plugins', name: 'plugins', component: { template: '<div>plugins</div>' } },
      { path: '/plugins/:id', name: 'plugin-detail', component: { template: '<div>plugin</div>' } },
      { path: '/render/templates/:templateId?', name: 'render-templates', component: { template: '<div>template</div>' } },
    ],
  })
}

describe('DashboardPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    feedbackMock.notifyError.mockClear()
    feedbackMock.notifySuccess.mockClear()
    feedbackMock.useToastFeedback.mockClear()
  })

  it('keeps passed checks collapsed and offers browser preparation', async () => {
    const router = createDashboardRouter()
    await router.push('/')
    await router.isReady()
    const { systemStore: store } = mockDashboardRefreshes()
    store.readiness = {
      status: 'degraded',
      checks: { config: 'ok', render: 'resource_missing' },
      issues: [{ code: 'platform.resource_missing', runtime_resources: ['chromium'], severity: 'warning', summary: '浏览器运行资源缺失', remediation: '准备运行环境后重试。' }],
    }
    const prepare = vi.spyOn(store, 'bootstrapManagedRuntime').mockResolvedValue({ task_id: 'fixture-runtime-task' })
    const wrapper = mount(DashboardPage, { global: { plugins: [getActivePinia()!, router] } })
    await flushPromises()
    expect(wrapper.get('details.readiness-check-group').attributes('open')).toBeUndefined()
    expect(wrapper.get('.readiness-check-group:not(details)').text()).toContain('图片生成')
    expect(wrapper.get('.readiness-check-group:not(details)').text()).toContain('缺少运行资源')
    expect(wrapper.get('.readiness-technical').attributes('open')).toBeUndefined()
    await wrapper.get('[data-testid="readiness-prepare-runtime"]').trigger('click')
    await flushPromises()
    expect(prepare).toHaveBeenCalledWith(['chromium'])
    store.readiness.issues = [{ code: 'database.unavailable', severity: 'error', summary: '数据库不可用' }]
    await flushPromises()
    expect(wrapper.find('[data-testid="readiness-prepare-runtime"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('renders a compact status page with overview cards, tabs, and bottom workbench cards', async () => {
    const router = createDashboardRouter()
    await router.push('/')
    await router.isReady()

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

    const wrapper = mount(DashboardPage, {
      global: {
        plugins: [getActivePinia()!, router],
      },
    })

    await flushPromises()

    expect(store.refreshAll).toHaveBeenCalledTimes(1)
    expect(adaptersStore.refresh).toHaveBeenCalledTimes(1)

    const backupButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('创建备份'))
    const diagnosticsButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('导出诊断包'))

    expect(backupButton).toBeTruthy()
    expect(diagnosticsButton).toBeTruthy()
    expect(wrapper.find('[data-testid="dashboard-connection-card"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="dashboard-overview-grid"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="dashboard-active-plugins-card"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('运行 1 / 失败 1')
    expect(wrapper.text()).toContain('数据库 schema 000001')

    await backupButton!.trigger('click')
    await diagnosticsButton!.trigger('click')

    expect(createBackupSpy).toHaveBeenCalledTimes(1)
    expect(exportDiagnosticsSpy).toHaveBeenCalledTimes(1)
  })

  it('updates the displayed uptime every second without refreshing dashboard data', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-12T00:00:00Z'))
    let wrapper: { text: () => string; unmount: () => void } | null = null

    try {
      const router = createDashboardRouter()
      await router.push('/')
      await router.isReady()

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

      wrapper = mount(DashboardPage, {
        global: {
          plugins: [getActivePinia()!, router],
        },
      })

      await flushPromises()

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

  it('opens the plugin list from the active plugins card', async () => {
    const router = createDashboardRouter()
    await router.push('/')
    await router.isReady()

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

    const wrapper = mount(DashboardPage, {
      global: {
        plugins: [getActivePinia()!, router],
      },
    })

    await flushPromises()
    await wrapper.get('[data-testid="dashboard-active-plugins-card"]').trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.name).toBe('plugins')
    expect(router.currentRoute.value.path).toBe('/plugins')
  })

  it('shows a protocol reminder when the protocol snapshot is degraded with transport issues', async () => {
    const router = createDashboardRouter()
    await router.push('/')
    await router.isReady()

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

    const wrapper = mount(DashboardPage, {
      global: {
        plugins: [getActivePinia()!, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).not.toContain('OneBot 主动连接已断开，正在重试。')
    expect(wrapper.text()).not.toContain('adapter.transport_forward_ws_session_lost')
    expect(feedbackMock.useToastFeedback).toHaveBeenCalledTimes(3)
    const protocolToastSource = feedbackMock.useToastFeedback.mock.calls[2][0] as { value: { message?: string | null } | null }
    expect(protocolToastSource.value?.message).toBe('协议提醒：OneBot11：OneBot 主动连接已断开，正在重试。')
  })

  it('renders readiness issues from the readiness snapshot', async () => {
    const router = createDashboardRouter()
    await router.push('/')
    await router.isReady()

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

    const wrapper = mount(DashboardPage, {
      global: {
        plugins: [getActivePinia()!, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('就绪检查')
    expect(toastMessages()).toContain('协议连接警告：OneBot authentication failed')
    expect(wrapper.text()).not.toContain('运行条件受限')
    expect(wrapper.findAll('[data-tone="success"]').length).toBeGreaterThan(0)
    expect(wrapper.text()).toContain('adapter.auth_failed')
    expect(wrapper.text()).toContain('请检查对应连接方式的访问令牌后重试连接。')
  })

  it('shows readiness issues and their recovery guidance', async () => {
    const router = createDashboardRouter()
    await router.push('/')
    await router.isReady()

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

    const wrapper = mount(DashboardPage, {
      global: {
        plugins: [getActivePinia()!, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('运行条件受限')
    expect(wrapper.text()).toContain('图片渲染 Chromium 尚未准备完成。')
    expect(toastMessages()).toContain('运行条件受限：图片渲染 Chromium 尚未准备完成。')
    expect(wrapper.text()).toContain('管理面可用')
  })

  it('deduplicates readiness issue codes already represented by issue cards', async () => {
    const router = createDashboardRouter()
    await router.push('/')
    await router.isReady()

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

    const wrapper = mount(DashboardPage, {
      global: {
        plugins: [getActivePinia()!, router],
      },
    })

    await flushPromises()

    expect(wrapper.findAll('.issues-list .issue-alert-card')).toHaveLength(1)
    expect(wrapper.text()).toContain('platform.resource_missing')
    expect((wrapper.text().match(/platform\.resource_missing/g) ?? []).length).toBe(1)
  })
})
