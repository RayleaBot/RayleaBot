import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'

import MenuCenterView from '@/views/builtin/MenuCenterView.vue'
import { useConfigStore } from '@/stores/config'
import { usePluginsStore } from '@/stores/plugins'
import type { ConfigDocument, ConfigUpdateResponse, PluginSummary } from '@/types/api'

vi.mock('@/adapter/feedback', () => ({
  notifySuccess: vi.fn(),
}))

function createConfig(): ConfigDocument {
  return {
    schema_version: '2',
    command: { prefixes: ['/'] },
    builtin_features: {
      menu: {
        commands: ['help', '帮助'],
        prefixes: [],
      },
    },
  } as ConfigDocument
}

function createPlugin(overrides: Partial<PluginSummary>): PluginSummary {
  return {
    id: 'weather',
    name: 'Weather',
    role: 'user',
    registration_state: 'installed',
    desired_state: 'enabled',
    runtime_state: 'running',
    display_state: 'running',
    source: {
      root: 'plugins/installed',
      verified: false,
    },
    trust: {
      level: 'third_party',
      label: '第三方',
    },
    commands: [],
    help: {
      groups: [],
    },
    command_conflicts: [],
    ...overrides,
  } as PluginSummary
}

describe('MenuCenterView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('previews menu commands in the DOM and saves builtin menu config', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ ok: true }), { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    const configStore = useConfigStore()
    const pluginsStore = usePluginsStore()
    configStore.document = createConfig()
    pluginsStore.items = [
      createPlugin({
        id: 'weather',
        name: 'Weather',
        commands: [
          {
            name: 'weather',
            aliases: ['天气'],
            description: '查询天气',
            usage: 'weather 上海',
            permission: 'everyone',
            command_source: 'manifest',
          },
        ],
        help: {
          summary: '天气菜单',
          groups: [
            {
              title: '查询',
              items: [
                {
                  title: '城市天气',
                  description: '查询城市天气',
                  usage: '/weather 上海',
                  command: 'weather',
                  permission: 'everyone',
                },
              ],
            },
          ],
        },
      }),
      createPlugin({
        id: 'raylea.echo',
        name: 'Echo',
        commands: [
          {
            name: 'echo',
            description: '复读收到的内容',
            command_source: 'manifest',
          },
        ],
        help: {
          summary: '复读菜单',
          groups: [],
        },
      }),
    ]

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(pluginsStore, 'fetchList').mockResolvedValue(undefined)
    const saveSpy = vi.spyOn(configStore, 'saveConfig').mockImplementation(async (nextConfig) => {
      configStore.document = nextConfig
      return {
        config: nextConfig,
        apply_effects: {
          applied_now: [],
          reloaded_now: [],
          restart_required_fields: [],
        },
        redacted_fields: [],
        restart_required: false,
      } as ConfigUpdateResponse
    })

    const wrapper = mount(MenuCenterView, {
      global: {
        plugins: [Antd],
      },
    })
    await flushPromises()

    expect(wrapper.get('[data-testid="menu-center-inherited-prefixes"]').text()).toContain('/')
    expect(wrapper.get('[data-testid="menu-center-root-preview"]').text()).toContain('/help')
    expect(wrapper.text()).toContain('/帮助')

    const pluginSelect = wrapper.getComponent('[data-testid="menu-center-plugin-select"]')
    await pluginSelect.vm.$emit('update:value', 'weather')
    await nextTick()
    expect(wrapper.text()).toContain('/help Weather')

    const commandSelect = wrapper.getComponent('[data-testid="menu-center-commands"]')
    const prefixSelect = wrapper.getComponent('[data-testid="menu-center-prefixes"]')
    await commandSelect.vm.$emit('update:value', ['menu', '菜单'])
    await prefixSelect.vm.$emit('update:value', ['#'])
    await nextTick()

    expect(wrapper.text()).toContain('#menu')
    expect(wrapper.text()).toContain('#menu Weather')
    expect(wrapper.text()).toContain('#Weather菜单')

    await wrapper.get('[data-testid="menu-center-save"]').trigger('click')
    await flushPromises()

    expect(saveSpy).toHaveBeenCalledWith(expect.objectContaining({
      builtin_features: {
        menu: {
          commands: ['menu', '菜单'],
          prefixes: ['#'],
        },
      },
    }))
    expect(fetchMock.mock.calls.some(([input]) => String(input).includes('/api/system/render/preview'))).toBe(false)
  })
})
