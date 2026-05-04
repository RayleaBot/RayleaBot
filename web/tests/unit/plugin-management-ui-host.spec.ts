import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import PluginManagementUIHost from '@/components/plugins/PluginManagementUIHost.vue'
import { useGovernanceStore } from '@/stores/governance'
import { usePluginsStore } from '@/stores/plugins'

function buildPlugin(overrides: Record<string, unknown> = {}) {
  return {
    id: 'example-config-panel',
    name: 'Example Config Panel',
    role: 'example',
    registration_state: 'installed',
    desired_state: 'disabled',
    runtime_state: 'stopped',
    display_state: 'disabled',
    version: '0.1.0',
    description: 'Python example plugin demonstrating config.read and config.write.',
    source: {
      root: 'examples/plugins',
      package_source_type: 'local_zip',
      package_source_ref: 'examples/plugins/example-config-panel.zip',
      verified: false,
    },
    trust: {
      level: 'unverified',
      label: '未验证来源',
    },
    default_config: {
      default_city: '北京',
      unit: 'celsius',
    },
    management_ui: {
      entry: 'web/index.html',
      label: '配置页面',
    },
    commands: [],
    command_conflicts: [],
    permissions: [],
    ...overrides,
  }
}

function assignIframeWindow(wrapper: ReturnType<typeof mount>) {
  const iframe = wrapper.get('[data-testid="plugin-management-ui-frame"]').element as HTMLIFrameElement
  const frameWindow = {
    postMessage: vi.fn(),
  } as unknown as Window

  Object.defineProperty(iframe, 'contentWindow', {
    configurable: true,
    value: frameWindow,
  })

  return {
    frameWindow,
    iframe,
  }
}

function dispatchBridgeMessage(source: MessageEventSource | null, data: unknown) {
  window.dispatchEvent(new MessageEvent('message', { data, source }))
}

describe('PluginManagementUIHost', () => {
  beforeEach(() => {
    window.localStorage.clear()
    setActivePinia(createPinia())
  })

  it('requires confirmation before loading an unverified plugin page', async () => {
    const pluginsStore = usePluginsStore()
    const governanceStore = useGovernanceStore()
    const fetchSettingsSpy = vi.spyOn(pluginsStore, 'fetchSettings').mockResolvedValue({
      plugin_id: 'example-config-panel',
      values: {
        default_city: '上海',
        unit: 'fahrenheit',
      },
    })

    const wrapper = mount(PluginManagementUIHost, {
      props: {
        plugin: buildPlugin(),
        title: '配置页面',
      },
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()

    expect(wrapper.get('[data-testid="plugin-management-ui-confirm"]').text()).toContain('未验证来源需要手动确认')
    expect(wrapper.find('[data-testid="plugin-management-ui-frame"]').exists()).toBe(false)
    expect(fetchSettingsSpy).not.toHaveBeenCalled()

    await wrapper.get('button').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="plugin-management-ui-confirm"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="plugin-management-ui-frame"]').exists()).toBe(true)
    expect(window.localStorage.getItem(
      'rayleabot.plugin-management-ui.confirmed:example-config-panel:0.1.0:local_zip:examples/plugins/example-config-panel.zip',
    )).toBe('1')
  })

  it('initializes, reloads, and saves settings through the bridge', async () => {
    const pluginsStore = usePluginsStore()
    const governanceStore = useGovernanceStore()
    const plugin = buildPlugin({
      source: {
        root: 'examples/plugins',
        package_source_type: 'local_directory',
        package_source_ref: 'examples/plugins/example-config-panel',
        verified: true,
      },
      trust: {
        level: 'third_party',
        label: '示例',
      },
    })

    const fetchSettingsSpy = vi.spyOn(pluginsStore, 'fetchSettings')
      .mockResolvedValueOnce({
        plugin_id: 'example-config-panel',
        values: {
          default_city: '上海',
          unit: 'fahrenheit',
        },
      })
      .mockResolvedValueOnce({
        plugin_id: 'example-config-panel',
        values: {
          default_city: '杭州',
          unit: 'celsius',
        },
      })
    const updateSettingsSpy = vi.spyOn(pluginsStore, 'updateSettings').mockResolvedValue({
      plugin_id: 'example-config-panel',
      changed_keys: ['default_city', 'unit'],
      values: {
        default_city: '深圳',
        unit: 'fahrenheit',
      },
    })
    const fetchDetailSpy = vi.spyOn(pluginsStore, 'fetchDetail').mockResolvedValue(plugin as never)
    const fetchCommandPolicySpy = vi.spyOn(governanceStore, 'fetchCommandPolicy').mockResolvedValue({
      default_level: 'everyone',
      cooldown: {
        user_command_rate_limit: '10/60s',
        group_command_rate_limit: '30/60s',
        cooldown_reply: true,
      },
      commands: [],
    })

    const wrapper = mount(PluginManagementUIHost, {
      props: {
        plugin,
        title: '配置页面',
      },
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()

    const { frameWindow } = assignIframeWindow(wrapper)

    dispatchBridgeMessage(frameWindow, {
      version: '1',
      source: 'plugin_management_ui',
      type: 'page.ready',
      request_id: 'req-ready',
    })
    await flushPromises()

    expect(fetchSettingsSpy).toHaveBeenCalledTimes(1)
    expect((frameWindow.postMessage as ReturnType<typeof vi.fn>).mock.calls[0]?.[0]).toMatchObject({
      version: '1',
      source: 'management_host',
      type: 'host.init',
      request_id: 'req-ready',
      payload: {
        plugin_id: 'example-config-panel',
        title: '配置页面',
        default_config: {
          default_city: '北京',
          unit: 'celsius',
        },
        settings: {
          default_city: '上海',
          unit: 'fahrenheit',
        },
      },
    })

    dispatchBridgeMessage(frameWindow, {
      version: '1',
      source: 'plugin_management_ui',
      type: 'settings.reload',
      request_id: 'req-reload',
    })
    await flushPromises()

    expect(fetchSettingsSpy).toHaveBeenCalledTimes(2)
    expect((frameWindow.postMessage as ReturnType<typeof vi.fn>).mock.calls[1]?.[0]).toMatchObject({
      version: '1',
      source: 'management_host',
      type: 'settings.changed',
      request_id: 'req-reload',
      payload: {
        changed_keys: [],
        values: {
          default_city: '杭州',
          unit: 'celsius',
        },
      },
    })

    dispatchBridgeMessage(frameWindow, {
      version: '1',
      source: 'plugin_management_ui',
      type: 'settings.save',
      request_id: 'req-save',
      payload: {
        values: {
          default_city: '深圳',
          unit: 'fahrenheit',
        },
      },
    })
    await flushPromises()

    expect(updateSettingsSpy).toHaveBeenCalledWith('example-config-panel', {
      default_city: '深圳',
      unit: 'fahrenheit',
    })
    expect(fetchDetailSpy).toHaveBeenCalledWith('example-config-panel')
    expect(fetchCommandPolicySpy).toHaveBeenCalledTimes(1)
    expect((frameWindow.postMessage as ReturnType<typeof vi.fn>).mock.calls[2]?.[0]).toMatchObject({
      version: '1',
      source: 'management_host',
      type: 'settings.changed',
      request_id: 'req-save',
      payload: {
        changed_keys: ['default_city', 'unit'],
        values: {
          default_city: '深圳',
          unit: 'fahrenheit',
        },
      },
    })
  })

  it('does not restart the iframe when unrelated plugin detail fields change', async () => {
    const pluginsStore = usePluginsStore()
    const plugin = buildPlugin({
      source: {
        root: 'examples/plugins',
        package_source_type: 'local_directory',
        package_source_ref: 'examples/plugins/example-config-panel',
        verified: true,
      },
      trust: {
        level: 'third_party',
        label: '示例',
      },
    })

    const fetchSettingsSpy = vi.spyOn(pluginsStore, 'fetchSettings').mockResolvedValue({
      plugin_id: 'example-config-panel',
      values: {
        default_city: '上海',
        unit: 'fahrenheit',
      },
    })

    const wrapper = mount(PluginManagementUIHost, {
      props: {
        plugin,
        title: '配置页面',
      },
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()

    const { frameWindow, iframe } = assignIframeWindow(wrapper)

    dispatchBridgeMessage(frameWindow, {
      version: '1',
      source: 'plugin_management_ui',
      type: 'page.ready',
      request_id: 'req-ready',
    })
    await flushPromises()

    expect(fetchSettingsSpy).toHaveBeenCalledTimes(1)

    await wrapper.setProps({
      plugin: {
        ...plugin,
        description: 'Updated description',
      },
    })
    await flushPromises()

    expect(fetchSettingsSpy).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-testid="plugin-management-ui-frame"]').element).toBe(iframe)
  })

  it('shows a retry surface when the iframe sends an invalid bridge message', async () => {
    const pluginsStore = usePluginsStore()
    vi.spyOn(pluginsStore, 'fetchSettings').mockResolvedValue({
      plugin_id: 'example-config-panel',
      values: {
        default_city: '上海',
        unit: 'fahrenheit',
      },
    })

    const wrapper = mount(PluginManagementUIHost, {
      props: {
        plugin: buildPlugin({
          source: {
            root: 'examples/plugins',
            package_source_type: 'local_directory',
            package_source_ref: 'examples/plugins/example-config-panel',
            verified: true,
          },
          trust: {
            level: 'third_party',
            label: '示例',
          },
        }),
        title: '配置页面',
      },
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()

    const { frameWindow } = assignIframeWindow(wrapper)

    dispatchBridgeMessage(frameWindow, {
      version: '1',
      source: 'plugin_management_ui',
      type: 'settings.delete',
      payload: {
        values: {},
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('插件页面未打开')
    expect(wrapper.text()).toContain('插件页面发送了无效消息，当前页面已停止交互。')
    expect(wrapper.find('[data-testid="vben-fallback"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('返回首页')
  })
})
