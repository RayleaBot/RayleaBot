import { createPinia, getActivePinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createRouter, createMemoryHistory } from 'vue-router'

import { notifyError, notifySuccess } from '@/adapter/feedback'
import PluginsPage from '@/views/plugins/PluginsView.vue'
import PluginIcon from '@/components/plugins/PluginIcon.vue'
import { usePluginsStore } from '@/stores/plugins'

vi.mock('@/adapter/feedback', () => ({
  notifyError: vi.fn(),
  notifySuccess: vi.fn(),
  useToastFeedback: vi.fn(),
}))

function testCommand(id: string, name: string, aliases: string[] = [], permission: 'everyone' | 'group_admin' | 'super_admin' = 'everyone') {
  return {
    id, name, effective_names: [name, ...aliases], description: `${name}说明`, usage: `/${name}`,
    permission, trigger: { type: 'exact' as const, names: [name, ...aliases] },
  }
}

describe('PluginsPage', () => {
  it('loads only the protected icon URL and restores the logo after an image error', async () => {
    const wrapper = mount(PluginIcon, { props: { pluginId: 'weather', icon: 'assets/weather.svg', version: '1.0.0' } })
    expect(wrapper.get('img').attributes('src')).toBe('/api/plugins/weather/icon')
    await wrapper.get('img').trigger('error')
    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.find('.raylea-mark').exists()).toBe(true)
    await wrapper.setProps({ icon: 'assets/replacement.svg' })
    expect(wrapper.find('img').exists()).toBe(true)
    await wrapper.setProps({ icon: undefined })
    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.find('.raylea-mark').exists()).toBe(true)
    wrapper.unmount()
  })

  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(notifyError).mockClear()
    vi.mocked(notifySuccess).mockClear()
  })

  it('calls enable action from the accessible icon toggle', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }],
    })
    const store = usePluginsStore()
    store.items = [{
      id: 'weather',
      name: 'Weather',
      role: 'community',
        state: 'disabled',
      commands: [],
      command_conflicts: [],
    }]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)
    const executeSpy = vi.spyOn(store, 'executeAction').mockResolvedValue(store.items[0])

    const wrapper = mount(PluginsPage, {
      global: {
        plugins: [getActivePinia()!, router],
      },
    })

    await flushPromises()
    const button = wrapper.find('[data-testid="plugin-enable-button-weather"]')
    expect(button.exists()).toBe(true)
    await button.trigger('click')

    expect(executeSpy).toHaveBeenCalledWith('weather', 'enable')
  })

  it('calls disable action from the same toggle when the plugin is enabled', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }],
    })
    const store = usePluginsStore()
    store.items = [{
      id: 'weather',
      name: 'Weather',
      role: 'community',
        state: 'running',
      commands: [],
      command_conflicts: [],
    }]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)
    const executeSpy = vi.spyOn(store, 'executeAction').mockResolvedValue(store.items[0])

    const wrapper = mount(PluginsPage, {
      global: {
        plugins: [getActivePinia()!, router],
      },
    })

    await flushPromises()
    const button = wrapper.find('[data-testid="plugin-enable-button-weather"]')
    expect(button.attributes('aria-checked')).toBe('true')
    await button.trigger('click')

    expect(executeSpy).toHaveBeenCalledWith('weather', 'disable')
  })

  it('keeps lifecycle switching plugins from sending duplicate actions', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }],
    })
    const store = usePluginsStore()
    store.items = [{
      id: 'weather',
      name: 'Weather',
      role: 'community',
      state: 'stopping',
      commands: [],
      command_conflicts: [],
    }]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)
    const executeSpy = vi.spyOn(store, 'executeAction').mockResolvedValue(store.items[0])

    const wrapper = mount(PluginsPage, {
      global: {
        plugins: [getActivePinia()!, router],
      },
    })

    await flushPromises()
    const toggle = wrapper.get('[data-testid="plugin-enable-button-weather"]')
    const reload = wrapper.get('[data-testid="plugin-reload-button-weather"]')

    expect(toggle.attributes('disabled')).toBeDefined()
    expect(toggle.attributes('aria-busy')).toBe('true')
    expect(reload.attributes('disabled')).toBeDefined()

    await toggle.trigger('click')
    await reload.trigger('click')

    expect(executeSpy).not.toHaveBeenCalled()
  })

  it('shows success feedback when reload action succeeds', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }],
    })
    const store = usePluginsStore()
    store.items = [{
      id: 'weather',
      name: 'Weather',
      role: 'community',
        state: 'running',
      commands: [],
      command_conflicts: [],
    }]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)
    const executeSpy = vi.spyOn(store, 'executeAction').mockResolvedValue(store.items[0])

    const wrapper = mount(PluginsPage, {
      global: {
        plugins: [getActivePinia()!, router],
      },
    })

    await flushPromises()
    await wrapper.get('[data-testid="plugin-reload-button-weather"]').trigger('click')
    await flushPromises()

    expect(executeSpy).toHaveBeenCalledWith('weather', 'reload')
    expect(notifySuccess).toHaveBeenCalledTimes(1)
    expect(notifyError).not.toHaveBeenCalled()
  })

  it('shows error feedback when reload action fails', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }],
    })
    const store = usePluginsStore()
    store.items = [{
      id: 'weather',
      name: 'Weather',
      role: 'community',
        state: 'running',
      commands: [],
      command_conflicts: [],
    }]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)
    const executeSpy = vi.spyOn(store, 'executeAction').mockRejectedValue(new Error('reload failed'))

    const wrapper = mount(PluginsPage, {
      global: {
        plugins: [getActivePinia()!, router],
      },
    })

    await flushPromises()
    await wrapper.get('[data-testid="plugin-reload-button-weather"]').trigger('click')
    await flushPromises()

    expect(executeSpy).toHaveBeenCalledWith('weather', 'reload')
    expect(notifyError).toHaveBeenCalledTimes(1)
    expect(notifySuccess).not.toHaveBeenCalled()
  })

  it('renders source, trust, and command conflict metadata', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/', component: { template: '<div />' } },
        { path: '/plugins/:id', name: 'plugin-detail', component: { template: '<div>plugin detail</div>' } },
      ],
    })
    const store = usePluginsStore()
    store.items = [
      {
        id: 'weather',
        name: 'Weather',
        version: '1.2.3',
        author: 'raylea',
        description: '提供当前城市天气与未来天气查询。',
        role: 'community',
        state: 'running',
        source: {
          root: 'plugins/installed',
          package_source_type: 'local_zip',
          package_source_ref: 'C:/plugins/weather.zip',
          verified: false,
        },
        trust: {
          level: 'unverified',
          label: '未验证来源',
        },
        commands: [
      { ...testCommand('fortune', '我的运势', ['今日运势', '每日运势']), trigger: { type: 'setting' as const, settings_key: 'fortune_command' } },
      testCommand('weather', '天气'),
      testCommand('weather-admin', '天气后台', [], 'group_admin'),
      testCommand('subscription-status', '订阅状态'),
      testCommand('subscription-refresh', '订阅刷新'),
        ],
    command_groups: [],
    help: {},
        command_conflicts: ['我的运势'],
      },
    ]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)

    const wrapper = mount(PluginsPage, {
      global: {
        plugins: [getActivePinia()!, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Weather')
    expect(wrapper.text()).toContain('1.2.3')
    expect(wrapper.find('.plugins-grid').text()).not.toContain('raylea')
    expect(wrapper.text()).toContain('提供当前城市天气与未来天气查询。')
    expect(wrapper.find('button[aria-label="查看概要"]').exists()).toBe(true)
    expect(wrapper.find('button[aria-label="管理"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('未验证来源')
    expect(wrapper.findAll('.plugin-card__meta .app-tag').filter(tag => tag.text() === '未验证来源')).toHaveLength(1)
    expect(wrapper.find('.plugin-health-notices').text()).not.toContain('未验证来源')
    store.items[0]!.trust = undefined
    await flushPromises()
    expect(wrapper.find('.plugin-health-notices').text()).toContain('未验证来源')
    expect(wrapper.find('.plugins-grid').text()).not.toContain('plugins/installed')
    expect(wrapper.text()).toContain('运行中')
    expect(wrapper.text()).toContain('1 个命令冲突')
    expect(wrapper.find('.plugins-grid').text()).not.toContain('我的运势')
    expect(wrapper.text()).not.toContain('fortune')
    expect(wrapper.text()).not.toContain('显示状态')
    expect(wrapper.text()).not.toContain('discovered')
    expect(wrapper.find('.plugins-grid').exists()).toBe(true)
    expect(wrapper.find('.plugin-card__meta').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('订阅状态')
    expect(wrapper.find('.plugin-health-notices').exists()).toBe(true)

    await wrapper.get('[data-testid="plugin-manage-button-weather"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value).toMatchObject({
      name: 'plugin-detail',
      params: { id: 'weather' },
      query: { panel: 'management-ui' },
    })
  })

  it('keeps verified third-party plugins in the community source filter', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }],
    })
    const store = usePluginsStore()
    store.items = [
      {
        id: 'official-help',
        name: 'Official Help',
        role: 'official',
        state: 'running',
        source: {
          root: 'plugins/installed/official-help',
          verified: true,
        },
        trust: {
          level: 'official',
          label: '官方插件',
        },
        commands: [],
        command_conflicts: [],
      },
      {
        id: 'verified-third-party',
        name: 'Verified Third Party',
        role: 'community',
        state: 'running',
        source: {
          root: 'plugins/installed/verified-third-party',
          verified: true,
        },
        trust: {
          level: 'third_party',
          label: '已验证第三方',
        },
        commands: [],
        command_conflicts: [],
      },
    ]

    const available = [...store.items]
    vi.spyOn(store, 'fetchList').mockImplementation(async query => {
      store.items = available.filter(item => !query?.source || (query.source === 'official' ? item.trust?.level === 'official' : item.trust?.level !== 'official'))
      store.total = store.items.length
    })

    const wrapper = mount(PluginsPage, {
      global: {
        plugins: [getActivePinia()!, router],
      },
    })

    await flushPromises()

    const sourceFilter = wrapper.getComponent('.filter-select')
    sourceFilter.vm.$emit('update:modelValue', 'community')
    await flushPromises()

    expect(store.fetchList).toHaveBeenLastCalledWith(expect.objectContaining({ source: 'community' }))
    expect(wrapper.find('.plugins-grid').text()).toContain('Verified Third Party')
    expect(wrapper.find('.plugins-grid').text()).not.toContain('Official Help')

    sourceFilter.vm.$emit('update:modelValue', 'official')
    await flushPromises()

    expect(wrapper.find('.plugins-grid').text()).toContain('Official Help')
    expect(wrapper.find('.plugins-grid').text()).not.toContain('Verified Third Party')
  })

  it('keeps all plugin commands accessible from the overview icon', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }],
    })
    const store = usePluginsStore()
    store.items = [
      {
        id: 'subscription-hub',
        name: '订阅中心',
        role: 'community',
        state: 'running',
        commands: [
      testCommand('status', '订阅状态'),
      testCommand('refresh', '订阅刷新'),
      testCommand('pause', '订阅暂停'),
      testCommand('resume', '订阅恢复'),
      testCommand('delete', '订阅删除'),
        ],
    command_groups: [],
    help: {},
        command_conflicts: [],
      },
    ]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)

    const wrapper = mount(PluginsPage, {
      global: {
        plugins: [getActivePinia()!, router],
      },
    })

    await flushPromises()

    expect(wrapper.get('.plugins-grid').text()).not.toContain('订阅恢复')
    await wrapper.get('button[aria-label="查看概要"]').trigger('click')
    await flushPromises()

    expect(document.body.textContent).toContain('订阅恢复')
    expect(document.body.textContent).toContain('订阅删除')
    wrapper.unmount()
  })

})
