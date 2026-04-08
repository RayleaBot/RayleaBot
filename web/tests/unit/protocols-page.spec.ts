import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ProtocolsPage from '@/pages/ProtocolsPage.vue'
import { useConfigStore } from '@/stores/config'
import { useProtocolLogsStore } from '@/stores/protocol-logs'
import { useSystemStore } from '@/stores/system'
import type { ConfigDocument } from '@/types/api'

function createFixtureConfig(): ConfigDocument {
  return {
    schema_version: '2',
    server: { host: '127.0.0.1', port: 8080 },
    onebot: {
      ws_url: '',
      access_token: '__REDACTED__',
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

  it('renders the protocol overview and protocol-only logs', async () => {
    const configStore = useConfigStore()
    const logsStore = useProtocolLogsStore()
    const systemStore = useSystemStore()

    configStore.document = createFixtureConfig()
    configStore.redactedFields = ['onebot.access_token']
    logsStore.items = [
      {
        timestamp: '2026-04-05T08:00:00Z',
        level: 'warn',
        protocol: 'onebot11',
        source: 'adapter',
        request_id: 'req_adapter_1',
        message: 'authentication failed for reverse websocket',
      },
    ]
    systemStore.system = {
      status: 'running',
      adapter_state: 'auth_failed',
      active_plugins: 1,
      uptime_seconds: 12,
    }
    systemStore.readiness = {
      status: 'degraded',
      issues: [
        {
          code: 'adapter.auth_failed',
          severity: 'warning',
          summary: 'OneBot authentication failed',
        },
      ],
    }

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(logsStore, 'fetchList').mockResolvedValue(undefined)

    const wrapper = mount(ProtocolsPage, {
      global: {
        plugins: [ElementPlus],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('协议中心')
    expect(wrapper.text()).toContain('OneBot11')
    expect(wrapper.text()).toContain('OneBot authentication failed')
    expect(wrapper.text()).toContain('反向 WebSocket 地址')
    expect(wrapper.text()).toContain('协议日志')
    expect(wrapper.text()).toContain('authentication failed for reverse websocket')
  })

  it('submits the full config document while editing protocol fields only', async () => {
    const configStore = useConfigStore()
    const logsStore = useProtocolLogsStore()
    const systemStore = useSystemStore()

    configStore.document = createFixtureConfig()
    systemStore.system = {
      status: 'running',
      adapter_state: 'connected',
      active_plugins: 1,
      uptime_seconds: 12,
    }
    systemStore.readiness = {
      status: 'ready',
    }

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(logsStore, 'fetchList').mockResolvedValue(undefined)
    const saveSpy = vi.spyOn(configStore, 'saveConfig').mockResolvedValue({
      config: configStore.document,
      redacted_fields: [],
      restart_required: true,
    })

    const wrapper = mount(ProtocolsPage, {
      global: {
        plugins: [ElementPlus],
      },
    })

    await flushPromises()

    const inputs = wrapper.findAll('input')
    const wsUrlInput = inputs.find((candidate) => candidate.attributes('aria-label') === '反向 WebSocket 地址' || candidate.element.value === '')
    expect(wsUrlInput).toBeTruthy()
    await wsUrlInput!.setValue('ws://127.0.0.1:8089/onebot')

    const saveButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('保存协议设置'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')

    expect(saveSpy).toHaveBeenCalledTimes(1)
    expect(saveSpy.mock.calls[0][0].onebot.ws_url).toBe('ws://127.0.0.1:8089/onebot')
    expect(saveSpy.mock.calls[0][0].server.host).toBe('127.0.0.1')
  })
})
