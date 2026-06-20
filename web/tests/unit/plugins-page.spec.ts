import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createRouter, createMemoryHistory } from 'vue-router'

import { notifyError, notifySuccess } from '@/adapter/feedback'
import PluginsPage from '@/views/plugins/PluginsView.vue'
import { usePluginsStore } from '@/stores/plugins'

vi.mock('@/adapter/feedback', () => ({
  notifyError: vi.fn(),
  notifySuccess: vi.fn(),
  useToastFeedback: vi.fn(),
}))

describe('PluginsPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(notifyError).mockClear()
    vi.mocked(notifySuccess).mockClear()
  })

  it('calls enable action when the chinese enable button is pressed', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }],
    })
    const store = usePluginsStore()
    store.items = [{
      id: 'weather',
      name: 'Weather',
      role: 'user',
        state: 'disabled',
      commands: [],
      command_conflicts: [],
    }]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)
    const executeSpy = vi.spyOn(store, 'executeAction').mockResolvedValue(store.items[0])

    const wrapper = mount(PluginsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()
    const button = wrapper.find('[data-testid="plugin-enable-button-weather"]')
    expect(button.exists()).toBe(true)
    expect(button.classes()).toContain('plugin-holo-button')
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
      role: 'user',
        state: 'running',
      commands: [],
      command_conflicts: [],
    }]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)
    const executeSpy = vi.spyOn(store, 'executeAction').mockResolvedValue(store.items[0])

    const wrapper = mount(PluginsPage, {
      global: {
        plugins: [Antd, router],
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
      role: 'user',
      state: 'stopping',
      commands: [],
      command_conflicts: [],
    }]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)
    const executeSpy = vi.spyOn(store, 'executeAction').mockResolvedValue(store.items[0])

    const wrapper = mount(PluginsPage, {
      global: {
        plugins: [Antd, router],
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
      role: 'user',
        state: 'running',
      commands: [],
      command_conflicts: [],
    }]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)
    const executeSpy = vi.spyOn(store, 'executeAction').mockResolvedValue(store.items[0])

    const wrapper = mount(PluginsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()
    await wrapper.get('[data-testid="plugin-reload-button-weather"]').trigger('click')
    await flushPromises()

    expect(executeSpy).toHaveBeenCalledWith('weather', 'reload')
    expect(notifySuccess).toHaveBeenCalledWith('操作已提交')
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
      role: 'user',
        state: 'running',
      commands: [],
      command_conflicts: [],
    }]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)
    const executeSpy = vi.spyOn(store, 'executeAction').mockRejectedValue(new Error('reload failed'))

    const wrapper = mount(PluginsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()
    await wrapper.get('[data-testid="plugin-reload-button-weather"]').trigger('click')
    await flushPromises()

    expect(executeSpy).toHaveBeenCalledWith('weather', 'reload')
    expect(notifyError).toHaveBeenCalledWith('操作未完成，请稍后重试。')
    expect(notifySuccess).not.toHaveBeenCalled()
  })

  it('renders source, trust, and command conflict metadata', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }],
    })
    const store = usePluginsStore()
    store.items = [
      {
        id: 'weather',
        name: 'Weather',
        version: '1.2.3',
        author: 'raylea',
        description: '提供当前城市天气与未来天气查询。',
        role: 'user',
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
          {
            name: '我的运势',
            aliases: ['今日运势', '每日运势'],
            description: '查看今日运势',
            usage: '我的运势',
            permission: 'everyone',
            command_source: 'dynamic',
            declaration_id: 'fortune',
          },
          {
            name: '天气',
            aliases: [],
            description: '查询天气',
            usage: '天气 上海',
            permission: 'member',
            command_source: 'manifest',
          },
          {
            name: '天气后台',
            aliases: [],
            description: '查询天气后台信息',
            usage: '天气后台',
            permission: 'group_admin',
            command_source: 'manifest',
          },
          {
            name: '订阅状态',
            aliases: [],
            description: '查看订阅状态',
            usage: '订阅状态',
            permission: 'member',
            command_source: 'manifest',
          },
          {
            name: '订阅刷新',
            aliases: [],
            description: '刷新订阅',
            usage: '订阅刷新',
            permission: 'member',
            command_source: 'manifest',
          },
        ],
        command_conflicts: ['我的运势'],
      },
    ]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)

    const wrapper = mount(PluginsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Weather')
    expect(wrapper.text()).toContain('1.2.3')
    expect(wrapper.text()).toContain('raylea')
    expect(wrapper.text()).toContain('提供当前城市天气与未来天气查询。')
    expect(wrapper.text()).toContain('操作')
    expect(wrapper.text()).toContain('未验证来源')
    expect(wrapper.text()).toContain('plugins/installed')
    expect(wrapper.text()).toContain('运行中')
    expect(wrapper.text()).toContain('1 个命令冲突')
    expect(wrapper.text()).toContain('我的运势')
    expect(wrapper.text()).toContain('2 个别名')
    expect(wrapper.text()).not.toContain('fortune')
    expect(wrapper.text()).not.toContain('显示状态')
    expect(wrapper.text()).not.toContain('discovered')
    expect(wrapper.find('.plugins-data-table').exists()).toBe(true)
    expect(wrapper.find('.plugin-cell-source').exists()).toBe(true)
    expect(wrapper.find('.plugin-cell-commands').exists()).toBe(true)
    expect(wrapper.findAll('.plugin-cell-commands .plugin-command-chip')).toHaveLength(3)
    expect(wrapper.text()).toContain('还有 2 个')
    expect(wrapper.text()).not.toContain('订阅状态')
    expect(wrapper.find('.plugin-health-notices').exists()).toBe(true)
  })

  it('keeps verified third-party plugins in the community source filter', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }],
    })
    const store = usePluginsStore()
    store.items = [
      {
        id: 'builtin-help',
        name: 'Builtin Help',
        role: 'builtin',
        state: 'running',
        source: {
          root: 'plugins/builtin/help',
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
        role: 'user',
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

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)

    const wrapper = mount(PluginsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    const sourceFilter = wrapper.getComponent('.filter-select')
    sourceFilter.vm.$emit('update:value', 'community')
    await flushPromises()

    expect(wrapper.find('.plugins-data-table').text()).toContain('Verified Third Party')
    expect(wrapper.find('.plugins-data-table').text()).not.toContain('Builtin Help')

    sourceFilter.vm.$emit('update:value', 'official')
    await flushPromises()

    expect(wrapper.find('.plugins-data-table').text()).toContain('Builtin Help')
    expect(wrapper.find('.plugins-data-table').text()).not.toContain('Verified Third Party')
  })

  it('expands and collapses overflow plugin commands in the list', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }],
    })
    const store = usePluginsStore()
    store.items = [
      {
        id: 'subscription-hub',
        name: '订阅中心',
        role: 'user',
        state: 'running',
        commands: [
          { name: '订阅状态', command_source: 'manifest' },
          { name: '订阅刷新', command_source: 'manifest' },
          { name: '订阅暂停', command_source: 'manifest' },
          { name: '订阅恢复', command_source: 'manifest' },
          { name: '订阅删除', command_source: 'manifest' },
        ],
        command_conflicts: [],
      },
    ]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)

    const wrapper = mount(PluginsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.findAll('.plugin-cell-commands .plugin-command-chip')).toHaveLength(3)
    expect(wrapper.text()).toContain('还有 2 个')
    expect(wrapper.text()).not.toContain('订阅恢复')

    const expandButton = wrapper.get('.plugin-command-expander')
    expect(expandButton.attributes('aria-expanded')).toBe('false')
    await expandButton.trigger('click')

    expect(wrapper.findAll('.plugin-cell-commands .plugin-command-chip')).toHaveLength(5)
    expect(wrapper.text()).toContain('订阅恢复')
    expect(wrapper.text()).toContain('收起')
    expect(wrapper.get('.plugin-command-expander').attributes('aria-expanded')).toBe('true')

    await wrapper.get('.plugin-command-expander').trigger('click')

    expect(wrapper.findAll('.plugin-cell-commands .plugin-command-chip')).toHaveLength(3)
    expect(wrapper.text()).not.toContain('订阅恢复')
  })

  it('uses a compact plugin table layout instead of the old metadata grid', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }],
    })
    const store = usePluginsStore()
    store.items = [
      {
        id: 'weather',
        name: 'Weather',
        role: 'user',
        state: 'running',
        commands: [],
        command_conflicts: [],
      },
    ]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)

    const wrapper = mount(PluginsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.find('.plugins-data-table').exists()).toBe(true)
    expect(wrapper.find('.plugin-cell-identity').exists()).toBe(true)
    expect(wrapper.find('.plugin-cell-status').exists()).toBe(true)
    expect(wrapper.find('.plugin-cell-actions').exists()).toBe(true)
    expect(wrapper.find('.plugin-summary-row').exists()).toBe(false)
    expect(wrapper.find('.desktop-table').exists()).toBe(false)
  })
})
