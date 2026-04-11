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
      access_token: '__REDACTED__',
      reverse_ws: { enabled: false, url: '' },
      forward_ws: { enabled: false, url: '' },
      http_api: { enabled: false, url: '' },
      webhook: { enabled: false, url: '' },
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
        { path: '/protocols', component: ProtocolsPage },
        { path: '/protocols/logs', component: { template: '<div>协议日志</div>' } },
      ],
    })
  }

  it('renders protocol settings with a readable chinese status summary', async () => {
    const configStore = useConfigStore()
    const protocolsStore = useProtocolsStore()

    configStore.document = createFixtureConfig()
    configStore.redactedFields = ['onebot.access_token']
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

    expect(wrapper.text()).toContain('协议中心')
    expect(wrapper.text()).toContain('OneBot11')
    expect(wrapper.text()).toContain('OneBot11 鉴权失败，请检查访问令牌')
    expect(wrapper.text()).toContain('传输状态')
    expect(wrapper.text()).toContain('主动连接 WebSocket')
    expect(wrapper.text()).toContain('主动连接鉴权失败')
    expect(wrapper.text()).toContain('连接设置')
    expect(wrapper.text()).toContain('查看协议日志')
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
    vi.spyOn(protocolsStore, 'refresh').mockResolvedValue({ snapshot: protocolsStore.snapshot! })
    const saveSpy = vi.spyOn(configStore, 'saveConfig').mockResolvedValue({
      config: configStore.document,
      redacted_fields: [],
      restart_required: true,
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

    const saveButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('保存协议设置'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')

    expect(saveSpy).toHaveBeenCalledTimes(1)
    expect(saveSpy.mock.calls[0][0].onebot.reverse_ws.url).toBe('wss://bot.example.com/reverse/onebot')
    expect(saveSpy.mock.calls[0][0].server.host).toBe('127.0.0.1')
  })
})
