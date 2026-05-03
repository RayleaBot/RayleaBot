import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { notifySuccess } from '@/adapter/feedback'
import PluginSettingsPage from '@/views/plugins/PluginSettingsView.vue'
import { useConfigStore } from '@/stores/config'
import type { ConfigDocument } from '@/types/api'

vi.mock('@/adapter/feedback', () => ({
  notifySuccess: vi.fn(),
}))

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
      auto_grant_capabilities: ['logger.write'],
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

describe('PluginSettingsPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('loads plugin-facing config fields and save metadata', async () => {
    const store = useConfigStore()
    store.document = createFixtureConfig()
    store.redactedFields = []
    store.applyEffects = {
      applied_now: ['command.prefixes', 'log.rate_limit_per_plugin'],
      reloaded_now: [],
      restart_required_fields: [],
    }
    store.restartRequired = false

    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)

    const wrapper = mount(PluginSettingsPage, {
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('插件设置')
    expect(wrapper.text()).toContain('命令入口')
    expect(wrapper.text()).toContain('插件授权')
    expect(wrapper.text()).toContain('日志保护')
    expect(wrapper.text()).toContain('插件存储')
    expect(wrapper.text()).toContain('命令前缀')
    expect(wrapper.text()).not.toContain('保存结果')
    expect(wrapper.text()).not.toContain('脱敏字段')
    expect(wrapper.text()).not.toContain('有未保存更改')
    expect(wrapper.text()).toContain('自动授权能力')
    expect(wrapper.text()).toContain('插件日志速率限制')
    expect(wrapper.text()).toContain('次数')
    expect(wrapper.text()).toContain('时间窗口')
    expect(wrapper.text()).toContain('单位')
    expect(wrapper.text()).toContain('当前表示')
    expect(wrapper.text()).not.toContain('插件消息速率限制')
    expect(wrapper.text()).toContain('插件工作目录软上限')
    expect(wrapper.text()).not.toContain('格式使用')
    expect(wrapper.find('.plugin-settings-nav-item').exists()).toBe(false)
    expect(wrapper.findAll('.plugin-settings-setting-row')).toHaveLength(4)
    expect(wrapper.find('[data-testid="plugin-settings-command-prefixes"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="plugin-settings-save"]').attributes('disabled')).toBeDefined()
  })

  it('submits plugin-facing config fields and shows compact save status', async () => {
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
            'command.prefixes',
            'permission.auto_grant_capabilities',
            'log.rate_limit_per_plugin',
            'storage.plugin_workdir_soft_limit_mb',
          ],
          reloaded_now: [],
          restart_required_fields: [],
        },
      }
    })

    const wrapper = mount(PluginSettingsPage, {
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()
    vi.useFakeTimers()

    const viewModel = wrapper.vm as unknown as {
      hasUnsavedChanges: boolean
      writeCommandPrefixTags: (value: unknown) => void
      writeField: (path: string, type: string, value: unknown) => void
    }
    expect(viewModel.hasUnsavedChanges).toBe(false)

    viewModel.writeCommandPrefixTags(['/', '!'])
    viewModel.writeField('permission.auto_grant_capabilities', 'list', 'logger.write\nmessage.send')
    viewModel.writeField('log.rate_limit_per_plugin', 'rateLimit', '300/10s')
    viewModel.writeField('storage.plugin_workdir_soft_limit_mb', 'number', 512)
    await flushPromises()

    expect(viewModel.hasUnsavedChanges).toBe(true)
    expect(wrapper.text()).toContain('有未保存更改')
    expect(wrapper.get('[data-testid="plugin-settings-save"]').attributes('disabled')).toBeUndefined()

    await wrapper.get('[data-testid="plugin-settings-save"]').trigger('click')
    await flushPromises()

    expect(saveSpy).toHaveBeenCalledTimes(1)
    const submitted = saveSpy.mock.calls[0][0]
    expect(submitted.command.prefixes).toEqual(['/', '!'])
    expect(submitted.permission.auto_grant_capabilities).toEqual(['logger.write', 'message.send'])
    expect(submitted.log.rate_limit_per_plugin).toBe('300/10s')
    expect(submitted.message.rate_limit_per_plugin).toBe('20/10s')
    expect(submitted.storage.plugin_workdir_soft_limit_mb).toBe(512)
    expect(submitted.server.host).toBe('127.0.0.1')
    expect(viewModel.hasUnsavedChanges).toBe(false)
    expect(wrapper.text()).not.toContain('有未保存更改')
    expect(wrapper.text()).toContain('保存完成，已生效')
    expect(wrapper.text()).not.toContain('保存结果')
    expect(notifySuccess).toHaveBeenCalledWith('配置已保存并已生效')

    vi.advanceTimersByTime(3000)
    await flushPromises()
    expect(wrapper.text()).not.toContain('保存完成，已生效')
  })
})
