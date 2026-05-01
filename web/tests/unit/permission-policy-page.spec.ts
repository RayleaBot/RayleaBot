import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { notifySuccess } from '@/adapter/feedback'
import PermissionPolicyPage from '@/views/operations/PermissionPolicyView.vue'
import { useConfigStore } from '@/stores/config'
import { useGovernanceStore } from '@/stores/governance'
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
      access_token: '__REDACTED__',
      reverse_ws: { enabled: false, url: '' },
      forward_ws: { enabled: false, url: '' },
      http_api: { enabled: false, url: '' },
      webhook: { enabled: false, url: '' },
    },
    database: { engine: 'sqlite', path: 'data/rayleabot.db' },
    command: { prefixes: ['/'] },
    admin: {
      super_admins: ['10001'],
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

function createRouterForPage() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/permission-policy', name: 'permission-policy', component: PermissionPolicyPage },
      { path: '/access-lists', name: 'access-lists', component: { template: '<div>access lists</div>' } },
    ],
  })
}

describe('PermissionPolicyPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    document.body.innerHTML = ''
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('loads config and command policy summary', async () => {
    const router = createRouterForPage()
    await router.push('/permission-policy')
    await router.isReady()

    const configStore = useConfigStore()
    const governanceStore = useGovernanceStore()
    configStore.document = createFixtureConfig()
    governanceStore.commandPolicy = {
      default_level: 'everyone',
      cooldown: {
        user_command_rate_limit: '10/60s',
        group_command_rate_limit: '30/60s',
        cooldown_reply: true,
      },
      commands: [],
    }

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(governanceStore, 'fetchCommandPolicy').mockResolvedValue(governanceStore.commandPolicy)

    const wrapper = mount(PermissionPolicyPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('权限策略')
    expect(wrapper.text()).not.toContain('配置超级管理员、默认权限级别和聊天命令速率限制。')
    expect(wrapper.text()).not.toContain('策略总览')
    expect(wrapper.text()).not.toContain('当前命令分发使用的权限级别、冷却和提示策略。')
    expect(wrapper.text()).not.toContain('这些配置参与聊天侧命令分发、权限判断和冷却判断。')
    expect(wrapper.text()).toContain('超级管理员')
    expect(wrapper.text()).toContain('所有成员')
    expect(wrapper.text()).toContain('60 秒内最多 10 次')
    expect(wrapper.text()).toContain('60 秒内最多 30 次')
    expect(wrapper.text()).toContain('冷却提示')
    expect(wrapper.text()).toContain('会发送提示')
    expect(wrapper.find('[data-testid="permission-policy-super-admins"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('保存结果')
    expect(wrapper.text()).not.toContain('有未保存更改')
    expect(wrapper.get('[data-testid="permission-policy-save"]').attributes('disabled')).toBeDefined()

    await wrapper.get('[data-testid="permission-policy-open-access-lists"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.name).toBe('access-lists')
  }, 15000)

  it('submits policy fields and refreshes command policy after saving', async () => {
    const router = createRouterForPage()
    await router.push('/permission-policy')
    await router.isReady()

    const configStore = useConfigStore()
    const governanceStore = useGovernanceStore()
    const config = createFixtureConfig()
    configStore.document = config
    governanceStore.commandPolicy = {
      default_level: 'everyone',
      cooldown: {
        user_command_rate_limit: '10/60s',
        group_command_rate_limit: '30/60s',
        cooldown_reply: true,
      },
      commands: [],
    }

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    const fetchPolicySpy = vi.spyOn(governanceStore, 'fetchCommandPolicy').mockResolvedValue(governanceStore.commandPolicy)
    const saveSpy = vi.spyOn(configStore, 'saveConfig').mockImplementation(async (submittedConfig) => {
      configStore.document = submittedConfig
      return {
        config: submittedConfig,
        redacted_fields: [],
        restart_required: false,
        apply_effects: {
          applied_now: ['permission.default_level', 'user.command_rate_limit', 'group.command_rate_limit', 'user.cooldown_reply'],
          reloaded_now: [],
          restart_required_fields: [],
        },
      }
    })

    const wrapper = mount(PermissionPolicyPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()
    vi.useFakeTimers()

    expect(wrapper.vm.superAdminCount).toBe(1)
    wrapper.vm.writeSuperAdminTags(['10001', '10002'])
    wrapper.vm.writeField('permission.default_level', 'select', 'group_admin')
    wrapper.vm.writeField('user.command_rate_limit', 'text', '20/60s')
    wrapper.vm.writeField('group.command_rate_limit', 'text', '60/60s')
    wrapper.vm.writeField('user.cooldown_reply', 'boolean', false)
    await flushPromises()

    expect(wrapper.vm.superAdminCount).toBe(1)
    expect(wrapper.vm.hasUnsavedChanges).toBe(true)
    expect(wrapper.text()).toContain('有未保存更改')
    expect(wrapper.get('[data-testid="permission-policy-save"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.text()).toContain('60 秒内最多 20 次')
    expect(wrapper.text()).toContain('60 秒内最多 60 次')

    await wrapper.get('[data-testid="permission-policy-save"]').trigger('click')
    await flushPromises()

    expect(saveSpy).toHaveBeenCalledTimes(1)
    const submitted = saveSpy.mock.calls[0][0]
    expect(submitted.admin.super_admins).toEqual(['10001', '10002'])
    expect(submitted.permission.default_level).toBe('group_admin')
    expect(submitted.user.command_rate_limit).toBe('20/60s')
    expect(submitted.group.command_rate_limit).toBe('60/60s')
    expect(submitted.user.cooldown_reply).toBe(false)
    expect(wrapper.vm.superAdminCount).toBe(2)
    expect(wrapper.vm.hasUnsavedChanges).toBe(false)
    expect(wrapper.text()).not.toContain('有未保存更改')
    expect(wrapper.text()).toContain('保存完成，已生效')
    expect(wrapper.text()).not.toContain('保存结果')
    expect(fetchPolicySpy).toHaveBeenCalledTimes(2)
    expect(notifySuccess).toHaveBeenCalledWith('配置已保存并已生效')

    vi.advanceTimersByTime(3000)
    await flushPromises()
    expect(wrapper.text()).not.toContain('保存完成，已生效')
  }, 15000)
})
