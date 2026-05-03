import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import ProtocolsPage from '@/views/protocols/ProtocolsView.vue'
import { useConfigStore } from '@/stores/config'
import { useProtocolsStore } from '@/stores/protocols'
import type { ConfigDocument } from '@/types/api'

function createFixtureConfig(): ConfigDocument {
  return {
    schema_version: '2',
    server: { host: '127.0.0.1', port: 8080 },
    onebot: {
      provider: 'standard',
      reverse_ws: { enabled: false, url: '', access_token: '' },
      forward_ws: { enabled: false, url: '', access_token: '' },
      http_api: { enabled: false, url: '', access_token: '' },
      webhook: { enabled: false, url: '', access_token: '' },
    },
    database: { engine: 'sqlite', path: 'data/rayleabot.db' },
    command: { prefixes: ['/'] },
    admin: {
      super_admins: [],
      session_ttl_days: 7,
      sliding_renewal: true,
      max_sessions: 3,
      login_fail_limit: 5,
      login_fail_window_seconds: 300,
    },
    permission: {
      default_level: 'everyone',
      auto_grant_capabilities: [],
    },
    render: {
      worker_count: 1,
      browser_args: ['--disable-gpu'],
      browser_path: '',
      timeout_seconds: 30,
      queue_wait_timeout_seconds: 15,
      queue_max_length: 32,
    },
    scheduler: {
      timezone: '',
    },
    runtime: {
      plugin_init_timeout_seconds: 30,
      plugin_init_max_total_seconds: 300,
      plugin_event_timeout_seconds: 60,
      max_pending_events_per_plugin: 16,
      max_pending_control_events_per_plugin: 4,
      nodejs_max_old_space_size_mb: 256,
      dependency_install_timeout_seconds: 900,
      max_concurrent_dependency_installs: 1,
      ipc_pending_actions_max: 256,
      ipc_action_burst_limit: '100/1s',
      stderr_rate_limit_bytes_per_second: 262144,
      max_concurrent_tasks_per_plugin: 4,
      crash_backoff_initial_seconds: 2,
      crash_backoff_max_seconds: 60,
      shutdown_grace_seconds: 10,
      ipc_message_max_bytes: 8388608,
    },
    storage: { kv_value_max_bytes: 65536, kv_total_limit_mb: 16, file_max_bytes: 10485760, plugin_workdir_soft_limit_mb: 256 },
    data: {
      audit_logs_retention_days: 90,
      event_records_retention_days: 7,
      download_cache_retention_days: 15,
    },
    log: { level: 'info', retention_days: 7, rate_limit_per_plugin: '200/10s' },
    message: {
      rate_limit_per_plugin: '20/10s',
      rate_limit_per_target: '5/5s',
      circuit_breaker_seconds: 30,
    },
    user: {
      command_rate_limit: '10/60s',
      cooldown_reply: true,
    },
    group: {
      command_rate_limit: '30/60s',
    },
    adapter: {
      connect_timeout_seconds: 15,
      reconnect_initial_seconds: 2,
      reconnect_multiplier: 2,
      reconnect_max_seconds: 120,
      reconnect_jitter_ratio: 0.2,
    },
    http: { timeout_seconds: 10, max_retries: 2, allow_private_hosts: [] },
    web: { exposure_mode: 'localhost_only', setup_local_only: true },
    backup: { default_consistency: 'offline' },
  }
}

describe('ProtocolsPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  function createTestRouter() {
    return createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/protocols', name: 'protocols', component: ProtocolsPage },
        { path: '/protocols/compatibility', name: 'protocols-compatibility', component: { template: '<div>compatibility</div>' } },
        { path: '/logs', name: 'logs', component: { template: '<div>logs</div>' } },
      ],
    })
  }

  it('renders protocol settings with a readable chinese status summary', async () => {
    const configStore = useConfigStore()
    const protocolsStore = useProtocolsStore()

    configStore.document = createFixtureConfig()
    configStore.redactedFields = []
    protocolsStore.snapshot = {
      protocol: 'onebot11',
      provider: 'standard',
      configured_transports: ['forward_ws'],
      active_transports: ['forward_ws'],
      transport_status: [
        { transport: 'reverse_ws', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
        { transport: 'forward_ws', enabled: true, configured: true, endpoint: 'ws://127.0.0.1:8089', state: 'auth_failed', summary: '主动连接鉴权失败' },
        { transport: 'http_api', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
        { transport: 'webhook', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
      ],
      readiness_status: 'degraded',
      summary: 'OneBot11 鉴权失败，请检查访问令牌',
      recent_transport_issues: [
        {
          code: 'adapter.transport_forward_ws_connection_failed',
          severity: 'warning',
          summary: 'OneBot 主动连接鉴权失败，请检查访问令牌。',
        },
      ],
    }

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(protocolsStore, 'refresh').mockResolvedValue({ snapshot: protocolsStore.snapshot! })

    const router = createTestRouter()
    await router.push('/protocols')
    await router.isReady()

    const wrapper = mount(ProtocolsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('协议中心')
    expect(wrapper.text()).toContain('OneBot11')
    expect(wrapper.text()).toContain('OneBot11 鉴权失败，请检查访问令牌')
    expect(wrapper.text()).toContain('传输状态')
    expect(wrapper.text()).toContain('传输异常')
    expect(wrapper.text()).toContain('主动连接 WebSocket')
    expect(wrapper.text()).toContain('主动连接鉴权失败')
    expect(wrapper.text()).toContain('adapter.transport_forward_ws_connection_failed')
    expect(wrapper.text()).toContain('OneBot 主动连接鉴权失败，请检查访问令牌。')
    expect(wrapper.text()).toContain('连接设置')
    expect(wrapper.findAll('input[aria-label="访问令牌"]')).toHaveLength(4)
    expect(wrapper.html()).not.toContain('__REDACTED__')
    expect(wrapper.text()).toContain('兼容矩阵')
    expect(wrapper.text()).toContain('查看实时日志')

    const realtimeLogsButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('查看实时日志'))
    expect(realtimeLogsButton).toBeTruthy()
    await realtimeLogsButton!.trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.name).toBe('logs')
    expect(router.currentRoute.value.query.protocol).toBe('onebot11')
  })

  it('hides the transport issue section after the issue list is cleared', async () => {
    const configStore = useConfigStore()
    const protocolsStore = useProtocolsStore()

    configStore.document = createFixtureConfig()
    protocolsStore.snapshot = {
      protocol: 'onebot11',
      provider: 'standard',
      configured_transports: ['forward_ws'],
      active_transports: ['forward_ws'],
      transport_status: [
        { transport: 'reverse_ws', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
        { transport: 'forward_ws', enabled: true, configured: true, endpoint: 'ws://127.0.0.1:8089', state: 'reconnecting', summary: '连接已断开，正在重试' },
        { transport: 'http_api', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
        { transport: 'webhook', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
      ],
      readiness_status: 'degraded',
      summary: 'OneBot11 传输链路部分可用',
      recent_transport_issues: [
        {
          code: 'adapter.transport_forward_ws_session_lost',
          severity: 'warning',
          summary: 'OneBot 主动连接已断开，正在重试。',
        },
      ],
    }

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(protocolsStore, 'refresh').mockResolvedValue({ snapshot: protocolsStore.snapshot! })

    const router = createTestRouter()
    await router.push('/protocols')
    await router.isReady()

    const wrapper = mount(ProtocolsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.find('[data-testid="protocol-issues"]').exists()).toBe(true)

    protocolsStore.snapshot = {
      ...protocolsStore.snapshot!,
      readiness_status: 'ready',
      recent_transport_issues: [],
    }
    await flushPromises()

    expect(wrapper.find('[data-testid="protocol-issues"]').exists()).toBe(false)
  })

  it('submits the full config document while editing protocol fields only', async () => {
    const configStore = useConfigStore()
    const protocolsStore = useProtocolsStore()

    configStore.document = createFixtureConfig()
    protocolsStore.snapshot = {
      protocol: 'onebot11',
      provider: 'standard',
      configured_transports: ['forward_ws'],
      active_transports: ['forward_ws'],
      transport_status: [
        { transport: 'reverse_ws', enabled: true, configured: true, endpoint: 'wss://bot.example.com/reverse', state: 'listening', summary: '等待 OneBot 回连' },
        { transport: 'forward_ws', enabled: true, configured: true, endpoint: 'ws://127.0.0.1:8089', state: 'connected', summary: '主动连接已建立' },
        { transport: 'http_api', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
        { transport: 'webhook', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
      ],
      readiness_status: 'ready',
      summary: 'OneBot11 主动连接已就绪',
      recent_transport_issues: [],
    }

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    const refreshSpy = vi.spyOn(protocolsStore, 'refresh')
      .mockResolvedValueOnce({ snapshot: protocolsStore.snapshot! })
      .mockImplementationOnce(async () => {
        protocolsStore.snapshot = {
          protocol: 'onebot11',
          provider: 'standard',
          configured_transports: ['reverse_ws', 'forward_ws'],
          active_transports: ['forward_ws'],
          transport_status: [
            { transport: 'reverse_ws', enabled: false, configured: true, endpoint: 'wss://bot.example.com/reverse/onebot', state: 'idle', summary: '未启用' },
            { transport: 'forward_ws', enabled: true, configured: true, endpoint: 'ws://127.0.0.1:8089', state: 'connected', summary: '主动连接已建立' },
            { transport: 'http_api', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
            { transport: 'webhook', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
          ],
          readiness_status: 'degraded',
          summary: 'OneBot11 等待回连',
          recent_transport_issues: [],
        }
        return { snapshot: protocolsStore.snapshot! }
      })
    const saveSpy = vi.spyOn(configStore, 'saveConfig').mockResolvedValue({
      config: configStore.document,
      redacted_fields: [],
      restart_required: false,
      apply_effects: {
        applied_now: [],
        reloaded_now: ['onebot.reverse_ws.url'],
        restart_required_fields: [],
      },
    })

    const router = createTestRouter()
    await router.push('/protocols')
    await router.isReady()

    const wrapper = mount(ProtocolsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    const wsUrlInput = wrapper.find('input[aria-label="回连地址"]')
    expect(wsUrlInput.exists()).toBe(true)
    await wsUrlInput.setValue('wss://bot.example.com/reverse/onebot')
    const tokenInputs = wrapper.findAll('input[aria-label="访问令牌"]')
    expect(tokenInputs).toHaveLength(4)
    await tokenInputs[0].setValue('reverse-secret')

    const saveButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('保存协议设置'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')

    expect(saveSpy).toHaveBeenCalledTimes(1)
    expect(refreshSpy).toHaveBeenCalledTimes(2)
    expect(saveSpy.mock.calls[0][0].onebot.reverse_ws.url).toBe('wss://bot.example.com/reverse/onebot')
    expect(saveSpy.mock.calls[0][0].onebot.reverse_ws.access_token).toBe('reverse-secret')
    expect(saveSpy.mock.calls[0][0].onebot.forward_ws.access_token).toBe('')
    expect('access_token' in saveSpy.mock.calls[0][0].onebot).toBe(false)
    expect(saveSpy.mock.calls[0][0].server.host).toBe('127.0.0.1')
    expect(wrapper.text()).toContain('未启用')
  })

  it('keeps cleared protocol numeric fields empty instead of forcing them to 0', async () => {
    const configStore = useConfigStore()
    const protocolsStore = useProtocolsStore()

    configStore.document = createFixtureConfig()
    protocolsStore.snapshot = {
      protocol: 'onebot11',
      provider: 'standard',
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
    }

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(protocolsStore, 'refresh').mockResolvedValue({ snapshot: protocolsStore.snapshot! })
    const saveSpy = vi.spyOn(configStore, 'saveConfig').mockResolvedValue({
      config: configStore.document,
      redacted_fields: [],
      restart_required: false,
      apply_effects: {
        applied_now: [],
        reloaded_now: ['adapter.connect_timeout_seconds'],
        restart_required_fields: [],
      },
    })

    const router = createTestRouter()
    await router.push('/protocols')
    await router.isReady()

    const wrapper = mount(ProtocolsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    const timeoutInput = wrapper.find('input[aria-label="连接超时（秒）"]')
    expect(timeoutInput.exists()).toBe(true)
    await timeoutInput.setValue('')

    const saveButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('保存协议设置'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')

    expect(saveSpy).toHaveBeenCalledTimes(1)
    expect(saveSpy.mock.calls[0][0].adapter.connect_timeout_seconds).toBeUndefined()
  })

  it('renders the apply effect summary on the protocol page', async () => {
    const configStore = useConfigStore()
    const protocolsStore = useProtocolsStore()

    configStore.document = createFixtureConfig()
    configStore.applyEffects = {
      applied_now: [],
      reloaded_now: ['onebot.forward_ws.url'],
      restart_required_fields: ['render.browser_args'],
    }
    configStore.restartRequired = true
    protocolsStore.snapshot = {
      protocol: 'onebot11',
      provider: 'standard',
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
    }

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(protocolsStore, 'refresh').mockResolvedValue({ snapshot: protocolsStore.snapshot! })

    const router = createTestRouter()
    await router.push('/protocols')
    await router.isReady()

    const wrapper = mount(ProtocolsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('保存结果')
    expect(wrapper.text()).toContain('已重载')
    expect(wrapper.text()).toContain('需重启生效')
    expect(wrapper.text()).toContain('onebot.forward_ws.url')
    expect(wrapper.text()).toContain('render.browser_args')
  })
})
