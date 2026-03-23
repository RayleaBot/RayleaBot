import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ConfigPage from '@/pages/ConfigPage.vue'
import { useConfigStore } from '@/stores/config'
import type { ConfigDocument } from '@/types/api'

function createFixtureConfig(): ConfigDocument {
  return {
    schema_version: '1',
    server: { host: '127.0.0.1', port: 8080 },
    onebot: {
      ws_url: 'ws://127.0.0.1:6700',
      access_token: '__REDACTED__',
      connect_timeout_seconds: 15,
      reconnect_initial_seconds: 2,
      reconnect_multiplier: 2,
      reconnect_max_seconds: 120,
      reconnect_jitter_ratio: 0.2,
    },
    database: { engine: 'sqlite', path: 'data/rayleabot.db' },
    storage: { kv_value_max_bytes: 65536, kv_total_limit_mb: 16, file_max_bytes: 10485760, plugin_workdir_soft_limit_mb: 256 },
    http: { timeout_seconds: 10, max_retries: 2, allow_private_hosts: [] },
    logging: { level: 'info', retention_days: 7, rate_limit_per_plugin: '200/10s' },
    auth: {
      super_admins: [],
      default_level: 'everyone',
      auto_grant_capabilities: [],
      session_ttl_days: 7,
      sliding_renewal: true,
      max_sessions: 3,
      login_fail_limit: 5,
      login_fail_window_seconds: 300,
    },
    runtime: {
      scheduler_timezone: '',
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
    render: {
      worker_count: 1,
      browser_args: ['--disable-gpu'],
      browser_path: '',
      timeout_seconds: 30,
      queue_wait_timeout_seconds: 15,
      queue_max_length: 32,
    },
    web: { exposure_mode: 'localhost_only', setup_local_only: true },
    backup: { default_consistency: 'offline' },
    retention: {
      audit_logs_retention_days: 90,
      event_records_retention_days: 7,
      download_cache_retention_days: 15,
    },
    command: { prefixes: ['/'] },
    cooldown: {
      user_command_rate_limit: '10/60s',
      group_command_rate_limit: '30/60s',
      cooldown_reply: true,
    },
  }
}

describe('ConfigPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('submits the edited config document', async () => {
    const store = useConfigStore()
    store.document = createFixtureConfig()
    store.redactedFields = ['onebot.access_token']

    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)
    const saveSpy = vi.spyOn(store, 'saveConfig').mockResolvedValue({
      config: store.document,
      redacted_fields: store.redactedFields,
      restart_required: true,
    })

    const wrapper = mount(ConfigPage, {
      global: {
        plugins: [ElementPlus],
      },
    })

    await flushPromises()
    const hostInput = wrapper.find('input')
    await hostInput.setValue('0.0.0.0')

    const saveButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('保存配置'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')

    expect(saveSpy).toHaveBeenCalledTimes(1)
    expect(saveSpy.mock.calls[0][0].server.host).toBe('0.0.0.0')
  })
})
