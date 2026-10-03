import { createPinia, getActivePinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import CommandsPage from '@/views/operations/CommandsView.vue'
import AppCollectionPagination from '@/components/AppCollectionPagination.vue'
import { useConfigStore } from '@/stores/config'
import { useGovernanceStore } from '@/stores/governance'
import { usePluginsStore } from '@/stores/plugins'
import { apiRequest } from '@/lib/http'
import { createConfigDocumentFixture } from './config-document.fixture'

vi.mock('@/lib/http', async importOriginal => ({ ...await importOriginal<typeof import('@/lib/http')>(), apiRequest: vi.fn() }))

function createFixtureConfig(prefixes: string[]) {
  return createConfigDocumentFixture((config) => { config.command.prefixes = prefixes })
}

describe('CommandsPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    document.body.innerHTML = ''
  })

  it('does not classify unloaded plugin commands as unavailable and opens an off-page deep link', async () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [
      { path: '/commands', name: 'commands', component: CommandsPage },
      { path: '/plugins/:id', name: 'plugin-detail', component: { template: '<div />' } },
    ] })
    await router.push('/commands')
    await router.isReady()
    const configStore = useConfigStore()
    configStore.document = createFixtureConfig(['/'])
    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    const governance = useGovernanceStore()
    governance.commandPolicy = { default_level: 'everyone', cooldown: { user_command_rate_limit: '10/60s', group_command_rate_limit: '30/60s', cooldown_reply: true }, commands: [] }
    vi.spyOn(governance, 'fetchCommandPolicy').mockResolvedValue(governance.commandPolicy)
    const makePlugin = (id: string) => ({ id, name: id, role: 'community', state: 'running', commands: [{ id, name: id, effective_names: [id], description: id + ' description', usage: id, permission: 'everyone', trigger: { type: 'exact', names: [id] } }], command_groups: [], command_conflicts: [], help: {} })
    const first = makePlugin('first-command')
    const later = makePlugin('later-command')
    governance.commandPolicy.commands = [{ plugin_id: later.id, plugin_name: later.name, command_id: later.id, command: later.id, aliases: [], trigger: { type: 'exact', names: [later.id] }, declared_permission: 'everyone', effective_permission: 'everyone', permission_source: 'declared' }]
    vi.mocked(apiRequest).mockImplementation(async path => {
      if (path === '/api/plugins/later-command') return { plugin: { ...later, webhooks: [] } } as never
      if (path.includes('cursor=1')) return { items: [later], total: 2 } as never
      return { items: [first], total: 2, next_cursor: '1' } as never
    })
    const wrapper = mount(CommandsPage, { global: { plugins: [getActivePinia()!, router] } })
    await flushPromises()
    expect(wrapper.text()).toContain('first-command description')
    // first-command has no policy entry, so its own declaration is shown translated, once.
    expect(wrapper.text()).toContain('所有人')
    expect(wrapper.text()).not.toContain('尚未生效')
    expect(wrapper.text()).not.toContain('later-command')
    expect(wrapper.getComponent(AppCollectionPagination).props('nextCursor')).toBe('1')
    expect(vi.mocked(apiRequest).mock.calls.some(call => call[0].includes('cursor='))).toBe(false)
    await router.push('/commands?plugin_id=later-command')
    await flushPromises()
    expect(wrapper.text()).toContain('later-command description')
    expect(wrapper.text()).toContain('当前可用')
    expect(vi.mocked(apiRequest).mock.calls.some(call => call[0] === '/api/plugins/later-command')).toBe(true)
    expect(vi.mocked(apiRequest).mock.calls.some(call => call[0].includes('cursor='))).toBe(false)
  })

  // Star Rail answers only to its own prefixes, so /体力 would never trigger; each usage starts with its plugin's prefix.
  it('starts each usage with the first prefix of its own plugin', async () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [
      { path: '/commands', name: 'commands', component: CommandsPage },
      { path: '/plugins/:id', name: 'plugin-detail', component: { template: '<div />' } },
    ] })
    await router.push('/commands')
    await router.isReady()
    const configStore = useConfigStore()
    configStore.document = createFixtureConfig(['!'])
    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    const governance = useGovernanceStore()
    governance.commandPolicy = { default_level: 'everyone', cooldown: { user_command_rate_limit: '10/60s', group_command_rate_limit: '30/60s', cooldown_reply: true }, commands: [] }
    vi.spyOn(governance, 'fetchCommandPolicy').mockResolvedValue(governance.commandPolicy)
    const makePlugin = (id: string, name: string, prefixes: string[], dedicated: string[]) => ({
      id, name: id, role: 'community', state: 'running', command_groups: [], command_conflicts: [], help: {},
      command_prefixes: prefixes, dedicated_command_prefixes: dedicated,
      commands: [{ id: name, name, effective_names: [name], description: name + ' 说明', usage: '#' + name, permission: 'everyone', trigger: { type: 'exact', names: [name] } }],
    })
    vi.mocked(apiRequest).mockResolvedValue({ items: [makePlugin('raylea.starrail', '体力', ['*', '星铁'], ['*', '星铁']), makePlugin('raylea.echo', 'echo', ['!'], [])], total: 2 } as never)

    const wrapper = mount(CommandsPage, { global: { plugins: [getActivePinia()!, router] } })
    await flushPromises()

    expect(wrapper.text()).toContain('*体力')
    expect(wrapper.text()).not.toContain('!体力')
    expect(wrapper.text()).toContain('!echo')
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
        role: 'official',
        state: 'running',
        commands: [
          {
            id: 'fortune',
            name: '我的运势',
            effective_names: ['我的运势', '今日运势'],
            description: '查看今日运势',
            usage: '我的运势',
            permission: 'everyone',
            trigger: { type: 'setting', settings_key: 'fortune_command' },
          },
        ],
        command_groups: [],
        help: {},
        command_conflicts: [],
      },
      {
        id: 'raylea.echo',
        name: 'Echo',
        role: 'official',
        state: 'disabled',
        commands: [
          {
            id: 'echo',
            name: 'echo',
            effective_names: ['echo'],
            description: '复读收到的内容',
            usage: '/echo',
            permission: 'everyone',
            trigger: { type: 'exact', names: ['echo'] },
          },
        ],
        command_groups: [],
        help: {},
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
          command_id: 'fortune',
          command: '我的运势',
          aliases: ['今日运势'],
          trigger: { type: 'setting', settings_key: 'fortune_command' },
          declared_permission: 'everyone',
          effective_permission: 'everyone',
          permission_source: 'declared',
        },
        {
          plugin_id: 'raylea.echo',
          plugin_name: 'Echo',
          command_id: 'echo',
          command: 'echo',
          aliases: [],
          trigger: { type: 'exact', names: ['echo'] },
          declared_permission: null,
          effective_permission: 'everyone',
          permission_source: 'default_level',
        },
      ],
    }

    store.rememberSummaries(store.items)
    vi.mocked(apiRequest).mockResolvedValue({ items: store.items, total: store.items.length })
    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(governanceStore, 'fetchCommandPolicy').mockResolvedValue(governanceStore.commandPolicy)

    const wrapper = mount(CommandsPage, {
      global: {
        plugins: [getActivePinia()!, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('指令中心')
    expect(wrapper.text()).toContain('指令列表')
    expect(wrapper.text()).toContain('插件设置中自定义')
    expect(wrapper.text()).toContain('所有人')
    // A declared permission is simply the level; only a level that follows the default says so.
    expect(wrapper.text()).not.toContain('跟随默认权限')
    expect(wrapper.text()).toContain('我的运势')
    expect(wrapper.text()).toContain('今日运势')
    expect(wrapper.text()).toContain('!我的运势')
    expect(wrapper.text()).toContain('当前可用')
    expect(wrapper.text()).toContain('权限策略')
    expect(router.currentRoute.value.fullPath).toContain('plugin_id=raylea.fortune')

    const select = wrapper.findComponent({ name: 'PluginPicker' })
    await select.vm.$emit('update:modelValue', ['raylea.echo'])
    await flushPromises()

    expect(router.currentRoute.value.fullPath).toContain('plugin_id=raylea.echo')
    expect(wrapper.text()).toContain('echo')
    expect(wrapper.text()).toContain('复读收到的内容')
    expect(wrapper.text()).toContain('跟随默认权限')
    expect(wrapper.text()).not.toContain('查看今日运势')

    const pluginLink = wrapper.find('.command-plugin-link')
    expect(pluginLink.attributes('href')).toBe('/plugins/raylea.echo')

    await wrapper.get('[data-testid="commands-open-permission-policy"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.name).toBe('permission-policy')
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
          command_id: 'ops',
          command: 'ops',
          aliases: ['ops-help'],
          trigger: { type: 'exact', names: ['ops', 'ops-help'] },
          declared_permission: null,
          effective_permission: 'everyone',
          permission_source: 'default_level',
        },
      ],
    }

    store.rememberSummaries(store.items)
    vi.mocked(apiRequest).mockResolvedValue({ items: store.items, total: store.items.length })
    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(governanceStore, 'fetchCommandPolicy').mockResolvedValue(governanceStore.commandPolicy)

    const wrapper = mount(CommandsPage, {
      global: {
        plugins: [getActivePinia()!, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('ops')
    expect(wrapper.text()).toContain('ops-help')
    expect(wrapper.text()).toContain('所有人')
    expect(wrapper.text()).toContain('未就绪')
    expect(wrapper.find('.command-plugin-link').attributes('href')).toBe('/plugins/ops.tools')
  }, 15000)
})
