import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { reactive } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import PluginManagementUIHost from '@/components/plugins/PluginManagementUIHost.vue'
import { useGovernanceStore } from '@/stores/governance'
import { usePluginsStore } from '@/stores/plugins'

function buildManagementPage(overrides: Record<string, unknown> = {}) {
  return {
    id: 'config',
    label: '配置页面',
    entry: 'web/index.html',
    ...overrides,
  }
}

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
      pages: [
        {
          id: 'config',
          label: '配置页面',
          entry: 'web/index.html',
        },
      ],
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

function parseFrameSrc(wrapper: ReturnType<typeof mount>) {
  const src = wrapper.get('[data-testid="plugin-management-ui-frame"]').attributes('src') ?? ''
  return new URL(src, 'https://rayleabot.local')
}

describe('PluginManagementUIHost', () => {
  beforeEach(() => {
    window.localStorage.clear()
    setActivePinia(createPinia())
    vi.unstubAllGlobals()
    vi.useRealTimers()
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
    vi.spyOn(pluginsStore, 'fetchSecrets').mockResolvedValue({
      plugin_id: 'example-config-panel',
      values: {},
    })

    const wrapper = mount(PluginManagementUIHost, {
      props: {
        plugin: buildPlugin(),
        title: '配置页面',
        page: buildManagementPage(),
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
    expect(wrapper.get('[data-testid="plugin-management-ui-frame"]').attributes('sandbox')).toBe('allow-forms allow-modals allow-scripts')
    expect(window.localStorage.getItem(
      'rayleabot.plugin-management-ui.confirmed:example-config-panel:0.1.0:local_zip:examples/plugins/example-config-panel.zip',
    )).toBe('1')
  })

  it('adds cache-busting metadata to the iframe src and changes it when the frame reloads', async () => {
    const pluginsStore = usePluginsStore()
    vi.spyOn(pluginsStore, 'fetchSettings').mockResolvedValue({
      plugin_id: 'example-config-panel',
      values: {
        default_city: '上海',
        unit: 'fahrenheit',
      },
    })
    vi.spyOn(pluginsStore, 'fetchSecrets').mockResolvedValue({
      plugin_id: 'example-config-panel',
      values: {},
    })

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
    const wrapper = mount(PluginManagementUIHost, {
      props: {
        plugin,
        title: '配置页面',
        page: buildManagementPage({
          id: 'secrets',
          label: '密钥设置',
          entry: 'web/secrets.html',
        }),
      },
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()

    const initialSrc = parseFrameSrc(wrapper)
    expect(initialSrc.pathname).toBe('/plugin-ui/example-config-panel/web/secrets.html')
    expect(initialSrc.searchParams.get('plugin_id')).toBe('example-config-panel')
    expect(initialSrc.searchParams.get('version')).toBe('0.1.0')
    expect(initialSrc.searchParams.get('entry')).toBe('web/secrets.html')
    expect(initialSrc.searchParams.get('source_ref')).toBe('examples/plugins/example-config-panel')
    const initialNonce = initialSrc.searchParams.get('nonce')
    expect(initialNonce).toBeTruthy()
    const initialSession = initialSrc.searchParams.get('session')
    expect(initialSession).toBeTruthy()

    await wrapper.setProps({
      plugin: {
        ...plugin,
        version: '0.1.1',
      },
    })
    await flushPromises()

    const retrySrc = parseFrameSrc(wrapper)
    expect(retrySrc.pathname).toBe('/plugin-ui/example-config-panel/web/secrets.html')
    expect(retrySrc.searchParams.get('version')).toBe('0.1.1')
    expect(retrySrc.searchParams.get('nonce')).not.toBe(initialNonce)
    expect(retrySrc.searchParams.get('session')).toBe(initialSession)
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
    const fetchSecretsSpy = vi.spyOn(pluginsStore, 'fetchSecrets')
      .mockResolvedValueOnce({
        plugin_id: 'example-config-panel',
        values: {
          api_token: 'secret-one',
        },
      })
      .mockResolvedValueOnce({
        plugin_id: 'example-config-panel',
        values: {
          api_token: 'secret-two',
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
    const updateSecretsSpy = vi.spyOn(pluginsStore, 'updateSecrets').mockResolvedValue({
      plugin_id: 'example-config-panel',
      changed_keys: ['api_token'],
      values: {
        api_token: 'secret-three',
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
        page: buildManagementPage({
          id: 'secrets',
          label: '密钥设置',
          entry: 'web/secrets.html',
        }),
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
    expect(fetchSecretsSpy).toHaveBeenCalledTimes(1)
    expect((frameWindow.postMessage as ReturnType<typeof vi.fn>).mock.calls[0]?.[0]).toMatchObject({
      version: '1',
      source: 'management_host',
      type: 'host.init',
      request_id: 'req-ready',
      payload: {
        plugin_id: 'example-config-panel',
        title: '配置页面',
        page: {
          id: 'secrets',
          label: '密钥设置',
          entry: 'web/secrets.html',
        },
        default_config: {
          default_city: '北京',
          unit: 'celsius',
        },
        settings: {
          default_city: '上海',
          unit: 'fahrenheit',
        },
        secrets: {
          api_token: 'secret-one',
        },
      },
    })

    dispatchBridgeMessage(frameWindow, {
      version: '1',
      source: 'plugin_management_ui',
      type: 'page.ready',
      request_id: 'req-ready-again',
    })
    await flushPromises()

    expect(fetchSettingsSpy).toHaveBeenCalledTimes(1)
    expect(fetchSecretsSpy).toHaveBeenCalledTimes(1)
    expect((frameWindow.postMessage as ReturnType<typeof vi.fn>).mock.calls[1]?.[0]).toMatchObject({
      version: '1',
      source: 'management_host',
      type: 'host.init',
      request_id: 'req-ready-again',
      payload: {
        plugin_id: 'example-config-panel',
        settings: {
          default_city: '上海',
          unit: 'fahrenheit',
        },
        secrets: {
          api_token: 'secret-one',
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
    expect((frameWindow.postMessage as ReturnType<typeof vi.fn>).mock.calls[2]?.[0]).toMatchObject({
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
    expect((frameWindow.postMessage as ReturnType<typeof vi.fn>).mock.calls[3]?.[0]).toMatchObject({
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

    dispatchBridgeMessage(frameWindow, {
      version: '1',
      source: 'plugin_management_ui',
      type: 'secrets.reload',
      request_id: 'req-secrets-reload',
    })
    await flushPromises()

    expect(fetchSecretsSpy).toHaveBeenCalledTimes(2)
    expect((frameWindow.postMessage as ReturnType<typeof vi.fn>).mock.calls[4]?.[0]).toMatchObject({
      version: '1',
      source: 'management_host',
      type: 'secrets.changed',
      request_id: 'req-secrets-reload',
      payload: {
        changed_keys: [],
        values: {
          api_token: 'secret-two',
        },
      },
    })

    dispatchBridgeMessage(frameWindow, {
      version: '1',
      source: 'plugin_management_ui',
      type: 'secrets.save',
      request_id: 'req-secrets-save',
      payload: {
        values: {
          api_token: 'secret-three',
        },
        deleted_keys: ['api_token_old'],
      },
    })
    await flushPromises()

    expect(updateSecretsSpy).toHaveBeenCalledWith('example-config-panel', {
      api_token: 'secret-three',
    }, ['api_token_old'])
    expect((frameWindow.postMessage as ReturnType<typeof vi.fn>).mock.calls[5]?.[0]).toMatchObject({
      version: '1',
      source: 'management_host',
      type: 'secrets.changed',
      request_id: 'req-secrets-save',
      payload: {
        changed_keys: ['api_token'],
        values: {
          api_token: 'secret-three',
        },
      },
    })
  })

  it('posts a plain page payload when the selected management page is reactive', async () => {
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

    vi.spyOn(pluginsStore, 'fetchSettings').mockResolvedValue({
      plugin_id: 'example-config-panel',
      values: {
        default_city: '上海',
      },
    })
    vi.spyOn(pluginsStore, 'fetchSecrets').mockResolvedValue({
      plugin_id: 'example-config-panel',
      values: {},
    })

    const wrapper = mount(PluginManagementUIHost, {
      props: {
        plugin,
        title: '密钥设置',
        page: reactive({
          id: 'secrets',
          label: '密钥设置',
          entry: 'web/secrets.html',
        }),
      },
      global: {
        plugins: [Antd],
      },
    })
    await flushPromises()

    const { frameWindow } = assignIframeWindow(wrapper)
    const deliveredMessages: unknown[] = []
    ;(frameWindow.postMessage as ReturnType<typeof vi.fn>).mockImplementation((message) => {
      deliveredMessages.push(structuredClone(message))
    })

    dispatchBridgeMessage(frameWindow, {
      version: '1',
      source: 'plugin_management_ui',
      type: 'page.ready',
      request_id: 'req-ready',
    })
    await flushPromises()

    expect(deliveredMessages).toHaveLength(1)
    expect(deliveredMessages[0]).toMatchObject({
      version: '1',
      source: 'management_host',
      type: 'host.init',
      request_id: 'req-ready',
      payload: {
        page: {
          id: 'secrets',
          label: '密钥设置',
          entry: 'web/secrets.html',
        },
      },
    })
    expect(wrapper.text()).not.toContain('插件页面未打开')
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
    vi.spyOn(pluginsStore, 'fetchSecrets').mockResolvedValue({
      plugin_id: 'example-config-panel',
      values: {},
    })

    const wrapper = mount(PluginManagementUIHost, {
      props: {
        plugin,
        title: '配置页面',
        page: buildManagementPage(),
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
    vi.spyOn(pluginsStore, 'fetchSecrets').mockResolvedValue({
      plugin_id: 'example-config-panel',
      values: {},
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
        page: buildManagementPage(),
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

  it('proxies authorized protocol targets and Bilibili user bridge requests', async () => {
    const pluginsStore = usePluginsStore()
    vi.spyOn(pluginsStore, 'fetchSettings').mockResolvedValue({
      plugin_id: 'raylea.subscription-hub',
      values: {},
    })
    vi.spyOn(pluginsStore, 'fetchSecrets').mockResolvedValue({
      plugin_id: 'raylea.subscription-hub',
      values: {},
    })
    const fetchSpy = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      if (url === '/api/protocols/onebot11/targets') {
        return new Response(JSON.stringify({
          protocol: 'onebot11',
          available: true,
          groups: [{ target_type: 'group', target_id: '5050', target_name: '测试群' }],
          private_users: [{ target_type: 'private', target_id: '2626', nickname: '测试用户' }],
          issues: [],
        }), { status: 200, headers: { 'content-type': 'application/json' } })
      }
      if (url === '/api/protocols/onebot11/identities/resolve') {
        expect(init?.method).toBe('POST')
        expect(JSON.parse(String(init?.body))).toEqual({
          items: [{ target_type: 'group', target_id: '5050', user_id: '10001' }],
        })
        return new Response(JSON.stringify({
          items: [{
            target_type: 'group',
            target_id: '5050',
            user_id: '10001',
            nickname: '测试号',
            group_nickname: '群名片',
            avatar_url: 'https://q1.qlogo.cn/g?b=qq&nk=10001&s=640',
          }],
          issues: [],
        }), { status: 200, headers: { 'content-type': 'application/json' } })
      }
      if (url === '/api/bilibili/users/resolve?query=%E6%B5%8B%E8%AF%95+UP') {
        return new Response(JSON.stringify({
          query: '测试 UP',
          exact: true,
          user: { uid: '1000001', name: '测试 UP', avatar_url: '' },
          candidates: [],
        }), { status: 200, headers: { 'content-type': 'application/json' } })
      }
      throw new Error(`unexpected fetch ${url}`)
    })
    vi.stubGlobal('fetch', fetchSpy)

    const plugin = buildPlugin({
      id: 'raylea.subscription-hub',
      role: 'builtin',
      source: {
        root: 'plugins/builtin/subscription_hub',
        package_source_type: 'local_directory',
        package_source_ref: 'plugins/builtin/subscription_hub',
        verified: true,
      },
      trust: {
        level: 'official',
        label: '内置',
      },
      permissions: [],
    })
    const wrapper = mount(PluginManagementUIHost, {
      props: {
        plugin,
        title: '订阅设置',
        page: buildManagementPage(),
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
      type: 'protocol.targets.reload',
      request_id: 'req-targets',
    })
    await flushPromises()
    expect((frameWindow.postMessage as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[0]).toMatchObject({
      type: 'protocol.targets.changed',
      request_id: 'req-targets',
      payload: {
        groups: [{ target_type: 'group', target_id: '5050', target_name: '测试群' }],
      },
    })

    dispatchBridgeMessage(frameWindow, {
      version: '1',
      source: 'plugin_management_ui',
      type: 'protocol.identities.resolve',
      request_id: 'req-identities',
      payload: {
        items: [{ target_type: 'group', target_id: '5050', user_id: '10001' }],
      },
    })
    await flushPromises()
    expect((frameWindow.postMessage as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[0]).toMatchObject({
      type: 'protocol.identities.resolved',
      request_id: 'req-identities',
      payload: {
        items: [{ group_nickname: '群名片' }],
      },
    })

    dispatchBridgeMessage(frameWindow, {
      version: '1',
      source: 'plugin_management_ui',
      type: 'bilibili.user.resolve',
      request_id: 'req-bili',
      payload: {
        query: '测试 UP',
      },
    })
    await flushPromises()
    expect((frameWindow.postMessage as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[0]).toMatchObject({
      type: 'bilibili.user.resolved',
      request_id: 'req-bili',
      payload: {
        exact: true,
        user: { uid: '1000001', name: '测试 UP' },
      },
    })
  })

  it('rejects protocol target bridge requests without granted capabilities', async () => {
    const pluginsStore = usePluginsStore()
    vi.spyOn(pluginsStore, 'fetchSettings').mockResolvedValue({
      plugin_id: 'example-config-panel',
      values: {},
    })
    vi.spyOn(pluginsStore, 'fetchSecrets').mockResolvedValue({
      plugin_id: 'example-config-panel',
      values: {},
    })
    const fetchSpy = vi.fn()
    vi.stubGlobal('fetch', fetchSpy)

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
          permissions: [],
        }),
        title: '配置页面',
        page: buildManagementPage(),
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
      type: 'protocol.targets.reload',
      request_id: 'req-denied',
    })
    await flushPromises()

    expect(fetchSpy).not.toHaveBeenCalled()
    expect((frameWindow.postMessage as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[0]).toMatchObject({
      type: 'error',
      request_id: 'req-denied',
      payload: {
        code: 'permission.denied',
      },
    })
  })
})
