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
      registration_state: 'installed',
      desired_state: 'disabled',
      runtime_state: 'stopped',
      display_state: 'disabled',
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
      registration_state: 'installed',
      desired_state: 'enabled',
      runtime_state: 'running',
      display_state: 'enabled',
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
      registration_state: 'installed',
      desired_state: 'enabled',
      runtime_state: 'running',
      display_state: 'enabled',
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
      registration_state: 'installed',
      desired_state: 'enabled',
      runtime_state: 'running',
      display_state: 'enabled',
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
        registration_state: 'installed',
        desired_state: 'enabled',
        runtime_state: 'running',
        display_state: 'discovered',
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
    expect(wrapper.find('.plugin-health-notices').exists()).toBe(true)
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
        registration_state: 'installed',
        desired_state: 'enabled',
        runtime_state: 'running',
        display_state: 'running',
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
