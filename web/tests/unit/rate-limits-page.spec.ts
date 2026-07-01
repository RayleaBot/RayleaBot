import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { notifySuccess } from '@/adapter/feedback'
import RateLimitsPage from '@/views/operations/RateLimitsView.vue'
import { useConfigStore } from '@/stores/config'
import type { ConfigDocument } from '@/types/api'

vi.mock('@/adapter/feedback', () => ({
  notifySuccess: vi.fn(),
  useToastFeedback: vi.fn(),
}))

function createFixtureConfig(): ConfigDocument {
  return {
    schema_version: '2',
    server: { host: '127.0.0.1', port: 8080 },
    onebot: {
      reverse_ws: { enabled: false, url: '', access_token: '' },
      forward_ws: { enabled: false, url: '', access_token: '' },
      http_api: { enabled: false, url: '', access_token: '' },
      webhook: { enabled: false, url: '', access_token: '' },
    },
    database: { engine: 'sqlite', path: 'data/rayleabot.db' },
    command: { prefixes: ['/'] },
    builtin_features: {
      menu: {
        commands: ['help', '帮助'],
        prefixes: [],
      },
    },
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
    },
    render: {
      worker_count: 1,
      browser_args: ['--disable-gpu'],
      browser_path: '',
      timeout_seconds: 30,
      queue_wait_timeout_seconds: 15,
      queue_max_length: 32,
      footer_template: 'Created By RayleaBot {{rayleabot_version}} & Plugin {{plugin_name}} {{plugin_version}}',
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

describe('RateLimitsPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('submits rate limit fields', async () => {
    const store = useConfigStore()
    store.document = createFixtureConfig()

    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)
    const saveSpy = vi.spyOn(store, 'saveConfig').mockImplementation(async (submittedConfig) => {
      store.document = submittedConfig
      return {
        config: submittedConfig,
        redacted_fields: [],
        restart_required: false,
        apply_effects: {
          applied_now: [
            'user.command_rate_limit',
            'group.command_rate_limit',
            'user.cooldown_reply',
            'message.rate_limit_per_plugin',
            'message.rate_limit_per_target',
          ],
          reloaded_now: [],
          restart_required_fields: [],
        },
      }
    })

    const wrapper = mount(RateLimitsPage, {
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()

    const viewModel = wrapper.vm as unknown as {
      hasUnsavedChanges: boolean
      writeField: (path: string, type: string, value: unknown) => void
    }

    expect(viewModel.hasUnsavedChanges).toBe(false)
    viewModel.writeField('user.command_rate_limit', 'rateLimit', '20/60s')
    viewModel.writeField('group.command_rate_limit', 'rateLimit', '60/60s')
    viewModel.writeField('user.cooldown_reply', 'boolean', false)
    viewModel.writeField('message.rate_limit_per_plugin', 'rateLimit', '30/10s')
    viewModel.writeField('message.rate_limit_per_target', 'rateLimit', '12/1m')
    await flushPromises()

    expect(viewModel.hasUnsavedChanges).toBe(true)
    expect(wrapper.get('[data-testid="rate-limits-save"]').attributes('disabled')).toBeUndefined()

    await wrapper.get('[data-testid="rate-limits-save"]').trigger('click')
    await flushPromises()

    expect(saveSpy).toHaveBeenCalledTimes(1)
    const submitted = saveSpy.mock.calls[0][0]
    expect(submitted.user.command_rate_limit).toBe('20/60s')
    expect(submitted.group.command_rate_limit).toBe('60/60s')
    expect(submitted.user.cooldown_reply).toBe(false)
    expect(submitted.message.rate_limit_per_plugin).toBe('30/10s')
    expect(submitted.message.rate_limit_per_target).toBe('12/1m')
    expect(viewModel.hasUnsavedChanges).toBe(false)
    expect(notifySuccess).toHaveBeenCalledTimes(1)
  })
})
