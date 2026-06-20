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
      reverse_ws: { enabled: false, url: '', access_token: '' },
      forward_ws: { enabled: false, url: '', access_token: '' },
      http_api: { enabled: false, url: '', access_token: '' },
      webhook: { enabled: false, url: '', access_token: '' },
    },
    database: { engine: 'sqlite', path: 'data/rayleabot.db' },
    command: { prefixes },
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

describe('CommandsPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    document.body.innerHTML = ''
  })

  it('renders a filtered command list with command and policy details', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/commands', name: 'commands', component: CommandsPage },
        { path: '/permission-policy', name: 'permission-policy', component: { template: '<div>permission policy</div>' } },
        { path: '/plugins/:id', name: 'plugin-detail', component: { template: '<div>plugin</div>' } },
      ],
    })
    await router.push('/commands?plugin_id=raylea.fortune')
    await router.isReady()

    const store = usePluginsStore()
    const configStore = useConfigStore()
    const governanceStore = useGovernanceStore()
    store.items = [
      {
        id: 'raylea.fortune',
        name: '运势',
        role: 'builtin',
        state: 'running',
        commands: [
          {
            name: '我的运势',
            aliases: ['今日运势'],
            description: '查看今日运势',
            usage: '我的运势',
            command_source: 'dynamic',
            declaration_id: 'fortune',
          },
        ],
        command_conflicts: [],
      },
      {
        id: 'raylea.echo',
        name: 'Echo',
        role: 'builtin',
        state: 'disabled',
        commands: [
          {
            name: 'echo',
            description: '复读收到的内容',
            command_source: 'manifest',
          },
        ],
        command_conflicts: [],
      },
    ]
    configStore.document = createFixtureConfig(['!'])
    governanceStore.commandPolicy = {
      default_level: 'everyone',
      cooldown: {
        user_command_rate_limit: '10/60s',
        group_command_rate_limit: '30/60s',
        cooldown_reply: true,
      },
      commands: [
        {
          plugin_id: 'raylea.fortune',
          plugin_name: '运势',
          command: '我的运势',
          aliases: ['今日运势'],
          command_source: 'dynamic',
          declaration_id: 'fortune',
          declared_permission: 'everyone',
          effective_permission: 'everyone',
          permission_source: 'declared',
        },
        {
          plugin_id: 'raylea.echo',
          plugin_name: 'Echo',
          command: 'echo',
          aliases: [],
          command_source: 'manifest',
          declared_permission: null,
          effective_permission: 'everyone',
          permission_source: 'default_level',
        },
      ],
    }

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)
    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(governanceStore, 'fetchCommandPolicy').mockResolvedValue(governanceStore.commandPolicy)

    const wrapper = mount(CommandsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('指令中心')
    expect(wrapper.text()).toContain('指令列表')
    expect(wrapper.text()).not.toContain('生效命令策略')
    expect(wrapper.text()).not.toContain('插件指令')
    expect(wrapper.text()).toContain('动态指令')
    expect(wrapper.text()).toContain('所有成员')
    expect(wrapper.text()).toContain('声明权限：所有成员')
    expect(wrapper.text()).toContain('权限来源：命令声明')
    expect(wrapper.text()).toContain('我的运势')
    expect(wrapper.text()).toContain('今日运势')
    expect(wrapper.text()).toContain('!我的运势')
    expect(wrapper.text()).toContain('当前可用')
    expect(wrapper.text()).toContain('权限策略')
    expect(wrapper.text()).not.toContain('治理摘要')
    expect(wrapper.text()).not.toContain('黑名单')
    expect(wrapper.text()).not.toContain('白名单')
    expect(router.currentRoute.value.fullPath).toContain('plugin_id=raylea.fortune')

    const select = wrapper.findComponent({ name: 'ASelect' })
    await select.vm.$emit('update:value', ['raylea.echo'])
    await flushPromises()

    expect(router.currentRoute.value.fullPath).toContain('plugin_id=raylea.echo')
    expect(wrapper.text()).toContain('echo')
    expect(wrapper.text()).toContain('复读收到的内容')
    expect(wrapper.text()).toContain('权限来源：默认权限')
    expect(wrapper.text()).not.toContain('查看今日运势')

    const pluginLink = wrapper.find('.command-plugin-link')
    expect(pluginLink.attributes('href')).toBe('/plugins/raylea.echo')

    await wrapper.get('[data-testid="commands-open-permission-policy"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.name).toBe('permission-policy')
  }, 15000)

  it('shows a single command empty state', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/commands', name: 'commands', component: CommandsPage },
        { path: '/permission-policy', name: 'permission-policy', component: { template: '<div>permission policy</div>' } },
        { path: '/plugins/:id', name: 'plugin-detail', component: { template: '<div>plugin</div>' } },
      ],
    })
    await router.push('/commands')
    await router.isReady()

    const store = usePluginsStore()
    const configStore = useConfigStore()
    const governanceStore = useGovernanceStore()

    store.items = []
    configStore.document = createFixtureConfig(['!'])
    governanceStore.commandPolicy = {
      default_level: 'everyone',
      cooldown: {
        user_command_rate_limit: '10/60s',
        group_command_rate_limit: '30/60s',
        cooldown_reply: true,
      },
      commands: [],
    }

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)
    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(governanceStore, 'fetchCommandPolicy').mockResolvedValue(governanceStore.commandPolicy)

    const wrapper = mount(CommandsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('指令列表')
    expect(wrapper.text()).toContain('暂无指令')
    expect(wrapper.text()).toContain('当前没有可展示的插件指令。')
    expect(wrapper.text()).not.toContain('暂无生效策略')
    expect(wrapper.text()).not.toContain('治理摘要')
    expect(wrapper.text()).not.toContain('白名单')
    expect(wrapper.text()).not.toContain('黑名单')
  }, 15000)

  it('shows policy-only commands when plugin rows are unavailable', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/commands', name: 'commands', component: CommandsPage },
        { path: '/permission-policy', name: 'permission-policy', component: { template: '<div>permission policy</div>' } },
        { path: '/plugins/:id', name: 'plugin-detail', component: { template: '<div>plugin</div>' } },
      ],
    })
    await router.push('/commands')
    await router.isReady()

    const store = usePluginsStore()
    const configStore = useConfigStore()
    const governanceStore = useGovernanceStore()

    store.items = []
    configStore.document = createFixtureConfig(['#'])
    governanceStore.commandPolicy = {
      default_level: 'everyone',
      cooldown: {
        user_command_rate_limit: '10/60s',
        group_command_rate_limit: '30/60s',
        cooldown_reply: true,
      },
      commands: [
        {
          plugin_id: 'ops.tools',
          plugin_name: 'Ops Tools',
          command: 'ops',
          aliases: ['ops-help'],
          command_source: 'manifest',
          declaration_id: undefined,
          declared_permission: null,
          effective_permission: 'everyone',
          permission_source: 'default_level',
        },
      ],
    }

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)
    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(governanceStore, 'fetchCommandPolicy').mockResolvedValue(governanceStore.commandPolicy)

    const wrapper = mount(CommandsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('ops')
    expect(wrapper.text()).toContain('ops-help')
    expect(wrapper.text()).toContain('所有成员')
    expect(wrapper.text()).toContain('未就绪')
    expect(wrapper.find('.command-plugin-link').attributes('href')).toBe('/plugins/ops.tools')
  }, 15000)
})
