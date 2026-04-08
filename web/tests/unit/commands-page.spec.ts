import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import CommandsPage from '@/pages/CommandsPage.vue'
import { usePluginsStore } from '@/stores/plugins'

describe('CommandsPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders flattened command rows and filters them by plugin selection', async () => {
    const store = usePluginsStore()
    store.items = [
      {
        id: 'weather',
        name: 'Weather',
        role: 'user',
        registration_state: 'installed',
        desired_state: 'enabled',
        runtime_state: 'running',
        commands: [
          {
            name: 'weather',
            aliases: ['tq'],
            description: '查询天气',
            usage: 'weather <城市>',
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
        commands: [
          {
            name: 'help',
            description: '查看帮助',
          },
        ],
        command_conflicts: [],
      },
    ]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)

    const wrapper = mount(CommandsPage, {
      global: {
        plugins: [ElementPlus],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('指令中心')
    expect(wrapper.text()).toContain('weather')
    expect(wrapper.text()).toContain('help')
    expect(wrapper.text()).toContain('当前可用')
    expect(wrapper.text()).toContain('已停用')
    expect(wrapper.find('.commands-data-table').exists()).toBe(true)

    const select = wrapper.findComponent({ name: 'ElSelect' })
    await select.vm.$emit('update:modelValue', ['weather'])
    await flushPromises()

    expect(wrapper.text()).toContain('weather')
    expect(wrapper.text()).not.toContain('查看帮助')
  })
})
