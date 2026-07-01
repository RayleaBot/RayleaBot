import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { notifySuccess } from '@/adapter/feedback'
import PermissionPolicyPage from '@/views/operations/PermissionPolicyView.vue'
import { useConfigStore } from '@/stores/config'
import { useGovernanceStore } from '@/stores/governance'
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
      super_admins: ['10001'],
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

    expect((wrapper.vm.configSections as Array<{ fields: Array<{ path: string }> }>).flatMap((section) => section.fields).map((field) => field.path)).not.toContain('user.cooldown_reply')
    expect(wrapper.find('[data-testid="permission-policy-super-admins"]').exists()).toBe(true)
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
          applied_now: ['permission.default_level'],
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

    expect(wrapper.vm.superAdminCount).toBe(1)
    wrapper.vm.writeSuperAdminTags(['10001', '10002'])
    wrapper.vm.writeField('permission.default_level', 'select', 'group_admin')
    await flushPromises()

    expect(wrapper.vm.superAdminCount).toBe(1)
    expect(wrapper.vm.hasUnsavedChanges).toBe(true)
    expect(wrapper.get('[data-testid="permission-policy-save"]').attributes('disabled')).toBeUndefined()

    await wrapper.get('[data-testid="permission-policy-save"]').trigger('click')
    await flushPromises()

    expect(saveSpy).toHaveBeenCalledTimes(1)
    const submitted = saveSpy.mock.calls[0][0]
    expect(submitted.admin.super_admins).toEqual(['10001', '10002'])
    expect(submitted.permission.default_level).toBe('group_admin')
    expect(submitted.user.command_rate_limit).toBe('10/60s')
    expect(submitted.group.command_rate_limit).toBe('30/60s')
    expect(submitted.user.cooldown_reply).toBe(true)
    expect(wrapper.vm.superAdminCount).toBe(2)
    expect(wrapper.vm.hasUnsavedChanges).toBe(false)
    expect(fetchPolicySpy).toHaveBeenCalledTimes(2)
    expect(notifySuccess).toHaveBeenCalledTimes(1)
  }, 15000)
})
