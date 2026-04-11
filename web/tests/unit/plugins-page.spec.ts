import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createRouter, createMemoryHistory } from 'vue-router'

import PluginsPage from '@/pages/PluginsPage.vue'
import { usePluginsStore } from '@/stores/plugins'

describe('PluginsPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
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
    await button.trigger('click')

    expect(executeSpy).toHaveBeenCalledWith('weather', 'enable')
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
            name: 'weather',
            aliases: ['tq', '天气'],
            description: '查询天气',
            usage: 'weather <城市>',
            permission: 'member',
          },
        ],
        command_conflicts: ['weather'],
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
    expect(wrapper.text()).toContain('未验证来源')
    expect(wrapper.text()).toContain('plugins/installed')
    expect(wrapper.text()).toContain('运行中')
    expect(wrapper.text()).toContain('1 个命令冲突')
    expect(wrapper.text()).toContain('weather')
    expect(wrapper.text()).toContain('2 个别名')
    expect(wrapper.text()).not.toContain('显示状态')
    expect(wrapper.text()).not.toContain('discovered')
    expect(wrapper.find('.plugins-data-table').exists()).toBe(true)
    expect(wrapper.find('.plugin-cell-source').exists()).toBe(true)
    expect(wrapper.find('.plugin-cell-commands').exists()).toBe(true)
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
