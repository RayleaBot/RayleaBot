import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import CommandsPage from '@/views/operations/CommandsView.vue'
import { useConfigStore } from '@/stores/config'
import { useGovernanceStore } from '@/stores/governance'
import { usePluginsStore } from '@/stores/plugins'
import type { ConfigDocument } from '@/types/api'

function createFixtureConfig(prefixes: string[]): ConfigDocument {
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
    command: { prefixes },
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

describe('CommandsPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders flattened command rows and filters them by plugin selection', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/commands', name: 'commands', component: CommandsPage },
        { path: '/plugins/:id', name: 'plugin-detail', component: { template: '<div>plugin</div>' } },
      ],
    })
    await router.push('/commands?plugin_id=weather')
    await router.isReady()

    const store = usePluginsStore()
    const configStore = useConfigStore()
    const governanceStore = useGovernanceStore()
    store.items = [
      {
        id: 'weather',
        name: 'Weather',
        role: 'user',
        registration_state: 'installed',
        desired_state: 'enabled',
        runtime_state: 'running',
        display_state: 'running',
        commands: [
          {
            name: 'weather',
            aliases: ['tq'],
            description: '查询天气',
            usage: '/weather <城市>',
          },
        ],
        command_conflicts: [],
      },
      {
        id: 'help',
        name: 'Help',
        role: 'builtin',
        registration_state: 'installed',
        desired_state: 'disabled',
        runtime_state: 'stopped',
        display_state: 'disabled',
        commands: [
          {
            name: 'help',
            description: '查看帮助',
          },
        ],
        command_conflicts: [],
      },
    ]
    configStore.document = createFixtureConfig(['!'])
    governanceStore.blacklist = {
      user_entries: [
        {
          entry_type: 'user',
          target_id: '10001',
          reason: '反复触发垃圾消息',
          created_at: '2026-04-17T09:00:00Z',
        },
      ],
      group_entries: [
        {
          entry_type: 'group',
          target_id: '20002',
          reason: '风险群已封禁',
          created_at: '2026-04-16T06:30:00Z',
        },
      ],
    }
    governanceStore.commandPolicy = {
      default_level: 'everyone',
      cooldown: {
        user_command_rate_limit: '10/60s',
        group_command_rate_limit: '30/60s',
        cooldown_reply: true,
      },
      commands: [
        {
          plugin_id: 'weather',
          plugin_name: 'Weather',
          command: 'weather',
          aliases: ['tq'],
          declared_permission: 'group_admin',
          effective_permission: 'group_admin',
          permission_source: 'declared',
        },
        {
          plugin_id: 'help',
          plugin_name: 'Help',
          command: 'help',
          aliases: [],
          declared_permission: null,
          effective_permission: 'everyone',
          permission_source: 'default_level',
        },
      ],
    }

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)
    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(governanceStore, 'refresh').mockResolvedValue({
      blacklist: governanceStore.blacklist,
      commandPolicy: governanceStore.commandPolicy,
    })

    const wrapper = mount(CommandsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('指令中心')
    expect(wrapper.text()).toContain('治理摘要')
    expect(wrapper.text()).toContain('黑名单')
    expect(wrapper.text()).toContain('生效命令策略')
    expect(wrapper.text()).toContain('全部声明命令')
    expect(wrapper.text()).toContain('默认权限')
    expect(wrapper.text()).toContain('所有成员')
    expect(wrapper.text()).toContain('10/60s')
    expect(wrapper.text()).toContain('30/60s')
    expect(wrapper.text()).toContain('10001')
    expect(wrapper.text()).toContain('20002')
    expect(wrapper.text()).toContain('群管理员')
    expect(wrapper.text()).toContain('命令声明')
    expect(wrapper.text()).toContain('weather')
    expect(wrapper.text()).toContain('!weather <城市>')
    expect(wrapper.text()).toContain('当前可用')
    expect(wrapper.find('.commands-data-table').exists()).toBe(true)
    expect(router.currentRoute.value.fullPath).toContain('plugin_id=weather')

    const select = wrapper.findComponent({ name: 'ASelect' })
    await select.vm.$emit('update:value', ['help'])
    await flushPromises()

    expect(router.currentRoute.value.fullPath).toContain('plugin_id=help')
    expect(wrapper.text()).toContain('help')
    expect(wrapper.text()).toContain('查看帮助')
    expect(wrapper.text()).not.toContain('查询天气')

    const pluginLink = wrapper.find('.command-plugin-link')
    expect(pluginLink.attributes('href')).toBe('/plugins/help')
  }, 15000)
})
