import { createPinia, getActivePinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import PluginManagementUIHost from '@/components/plugins/PluginManagementUIHost.vue'
import { useConfigStore } from '@/stores/config'
import { usePluginsStore } from '@/stores/plugins'
import { useUiShellStore } from '@/stores/ui-shell'
import type { PluginDetail } from '@/types/api'

class FakeMessagePort {
  readonly sent: unknown[] = []
  private readonly listeners = new Set<(event: MessageEvent) => void>()
  closed = false

  addEventListener(type: string, listener: EventListenerOrEventListenerObject) {
    if (type !== 'message') return
    const handler = typeof listener === 'function'
      ? listener as (event: MessageEvent) => void
      : (event: MessageEvent) => listener.handleEvent(event)
    this.listeners.add(handler)
  }

  postMessage(message: unknown) {
    this.sent.push(structuredClone(message))
  }

  start() {}

  close() {
    this.closed = true
  }

  emit(message: unknown) {
    const event = new MessageEvent('message', { data: message })
    for (const listener of this.listeners) listener(event)
  }
}

class FakeMessageChannel {
  static latest: FakeMessageChannel | null = null
  readonly port1 = new FakeMessagePort()
  readonly port2 = new FakeMessagePort()

  constructor() {
    FakeMessageChannel.latest = this
  }
}

function buildManagementPage() {
  return { id: 'config', label: '配置页面' }
}

function buildPlugin(overrides: Record<string, unknown> = {}): PluginDetail {
  return {
    id: 'example-config-panel',
    name: 'Example Config Panel',
    role: 'community',
    state: 'disabled',
    version: '0.2.0',
    description: 'Go example plugin with a Vue management page.',
    source: {
      root: 'examples/plugins',
      package_source_type: 'local_zip',
      package_source_ref: 'examples/plugins/example-config-panel.zip',
      verified: true,
    },
    trust: { level: 'third_party', label: '示例' },
    management_ui: { entry: 'ui/index.html', pages: [buildManagementPage()] },
    commands: [],
    command_groups: [],
    help: {},
    command_conflicts: [],
    permissions: {},
    webhooks: [],
    ...overrides,
  } as unknown as PluginDetail
}

function configureStores() {
  const configStore = useConfigStore()
  configStore.document = {
    web: {
      plugin_ui_origin_template: 'https://{plugin_host}.plugins.example.test',
    },
  } as never

  const pluginsStore = usePluginsStore()
  vi.spyOn(pluginsStore, 'fetchSettings').mockResolvedValue({
    plugin_id: 'example-config-panel',
    values: { default_city: '上海', unit: 'fahrenheit' },
  })
  return pluginsStore
}

function mountHost(plugin = buildPlugin()) {
  return mount(PluginManagementUIHost, {
    props: { plugin, title: '配置页面', page: buildManagementPage() },
    global: { plugins: [getActivePinia()!] },
  })
}

function assignIframeWindow(wrapper: ReturnType<typeof mountHost>) {
  const iframe = wrapper.get('[data-testid="plugin-management-ui-frame"]').element as HTMLIFrameElement
  const origin = new URL(iframe.src).origin
  const frameWindow = { postMessage: vi.fn() } as unknown as Window
  Object.defineProperty(iframe, 'contentWindow', { configurable: true, value: frameWindow })
  return { iframe, frameWindow, origin }
}

function dispatchWindowMessage(source: Window, origin: string, data: unknown) {
  const event = new MessageEvent('message', { data, origin })
  Object.defineProperty(event, 'source', { configurable: true, value: source })
  window.dispatchEvent(event)
}

function installWebPlatformMocks() {
  const digest = Uint8Array.from({ length: 32 }, (_, index) => index + 1)
  vi.stubGlobal('crypto', {
    getRandomValues: (target: Uint8Array) => {
      target.forEach((_, index) => { target[index] = (index + 17) % 256 })
      return target
    },
    subtle: { digest: vi.fn(async () => digest.buffer) },
  })
  vi.stubGlobal('MessageChannel', FakeMessageChannel)
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input)
    if (path === '/api/plugins/example-config-panel/secrets' && (!init?.method || init.method === 'GET')) {
      return new Response(JSON.stringify({
        plugin_id: 'example-config-panel',
        configured: { api_token: true, optional_token: false },
      }), { status: 200, headers: { 'content-type': 'application/json' } })
    }
    if (path === '/api/plugins/example-config-panel/secrets' && init?.method === 'PUT') {
      return new Response(JSON.stringify({
        plugin_id: 'example-config-panel',
        configured: { api_token: true, optional_token: true },
        changed_keys: ['optional_token'],
      }), { status: 200, headers: { 'content-type': 'application/json' } })
    }
    if (path === '/api/plugins/example-config-panel/secrets' && init?.method === 'DELETE') {
      return new Response(JSON.stringify({
        plugin_id: 'example-config-panel',
        configured: { api_token: false, optional_token: true },
        changed_keys: ['api_token'],
      }), { status: 200, headers: { 'content-type': 'application/json' } })
    }
    throw new Error(`unexpected fetch ${path}`)
  }))
}

async function connectBridge(wrapper: ReturnType<typeof mountHost>) {
  const { iframe, frameWindow, origin } = assignIframeWindow(wrapper)
  const src = new URL(iframe.src)
  const nonce = src.searchParams.get('bridge_nonce')
  expect(nonce).toMatch(/^nonce-[a-f0-9]{48}$/)

  dispatchWindowMessage(frameWindow, origin, {
    version: '3',
    source: 'plugin_management_ui',
    type: 'page.ready',
    nonce,
  })
  await flushPromises()

  const channel = FakeMessageChannel.latest
  expect(channel).not.toBeNull()
  expect(frameWindow.postMessage).toHaveBeenCalledWith({
    version: '3',
    source: 'management_host',
    type: 'host.connect',
    nonce,
  }, origin, [channel?.port2])
  return { iframe, frameWindow, origin, channel: channel! }
}

describe('PluginManagementUIHost bridge v3', () => {
  afterEach(() => vi.useRealTimers())
  beforeEach(() => {
    window.localStorage.clear()
    setActivePinia(createPinia())
    FakeMessageChannel.latest = null
    vi.unstubAllGlobals()
    installWebPlatformMocks()
    configureStores()
  })

  it('requires an explicit adapter for target queries and sends only that instance path', async () => {
    const wrapper = mountHost(buildPlugin({ permissions: { 'group.list': {}, 'friend.list': {} } }))
    await flushPromises()
    const { channel } = await connectBridge(wrapper)
    const fetch = vi.mocked(window.fetch)
    fetch.mockClear()
    channel.port1.emit({ version: '3', source: 'plugin_management_ui', type: 'protocol.targets.reload', request_id: 'missing-selector', payload: {} })
    await flushPromises()
    expect(fetch).not.toHaveBeenCalled()
    expect(channel.port1.sent.at(-1)).toMatchObject({ type: 'error', request_id: 'missing-selector', payload: { code: 'platform.invalid_request' } })

    fetch.mockResolvedValueOnce(new Response(JSON.stringify({ protocol: 'onebot11', available: true, groups: [], private_users: [], issues: [] }), { status: 200, headers: { 'content-type': 'application/json' } }))
    channel.port1.emit({ version: '3', source: 'plugin_management_ui', type: 'protocol.targets.reload', request_id: 'second-selector', payload: { adapter_id: 'second' } })
    await flushPromises()
    expect(fetch).toHaveBeenCalledTimes(1)
    expect(fetch.mock.calls[0][0]).toBe('/api/adapters/second/onebot11/targets')
    expect(channel.port1.sent.at(-1)).toMatchObject({ type: 'protocol.targets.changed', request_id: 'second-selector' })
    wrapper.unmount()
  })


  it('requires an explicit confirmation before loading an unverified plugin page', async () => {
    const pluginsStore = usePluginsStore()
    const plugin = buildPlugin({
      trust: { level: 'unverified', label: '未验证来源' },
      source: {
        root: 'examples/plugins',
        package_source_type: 'local_zip',
        package_source_ref: 'examples/plugins/example-config-panel.zip',
        verified: false,
      },
    })
    const wrapper = mountHost(plugin)
    await flushPromises()

    expect(wrapper.get('[data-testid="plugin-management-ui-confirm"]').text()).toContain('未验证来源需要手动确认')
    expect(wrapper.find('[data-testid="plugin-management-ui-frame"]').exists()).toBe(false)
    expect(pluginsStore.fetchSettings).not.toHaveBeenCalled()

    await wrapper.get('[data-testid="plugin-management-ui-confirm"] button').trigger('click')
    await flushPromises()

    const iframe = wrapper.get('[data-testid="plugin-management-ui-frame"]')
    expect(iframe.attributes('sandbox')).toBe('allow-forms allow-same-origin allow-scripts')
    expect(window.localStorage.getItem(
      'rayleabot.plugin-management-ui.confirmed:example-config-panel:0.2.0:examples/plugins/example-config-panel.zip',
    )).toBe('1')
    wrapper.unmount()
  })

  it('loads the Vue artifact from an independent plugin origin', async () => {
    const wrapper = mountHost()
    await flushPromises()

    const src = new URL(wrapper.get('[data-testid="plugin-management-ui-frame"]').attributes('src'))
    expect(src.origin).toBe('https://p-0102030405060708.plugins.example.test')
    expect(src.origin).not.toBe(window.location.origin)
    expect(src.pathname).toBe('/index.html')
    expect(src.searchParams.get('version')).toBe('0.2.0')
    expect(src.searchParams.get('bridge_nonce')).toMatch(/^nonce-[a-f0-9]{48}$/)
    wrapper.unmount()
  })

  it('explains how to fix a plugin page origin that matches the management origin', async () => {
    const configStore = useConfigStore()
    configStore.document = {
      web: { plugin_ui_origin_template: window.location.origin },
    } as never

    const wrapper = mountHost()
    await flushPromises()

    expect(wrapper.get('[role="alert"]').text()).toContain('插件页面域不能与管理域相同')
    expect(wrapper.get('[role="alert"]').text()).toContain('插件页面域模板')
    expect(wrapper.find('[data-testid="plugin-management-ui-frame"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('restarts the plugin page after a runtime reload reaches running', async () => {
    const wrapper = mountHost(buildPlugin({ state: 'running' }))
    await flushPromises()
    const firstSrc = new URL(wrapper.get('[data-testid="plugin-management-ui-frame"]').attributes('src'))

    await wrapper.setProps({ plugin: buildPlugin({ state: 'starting' }) })
    await flushPromises()
    const startingSrc = new URL(wrapper.get('[data-testid="plugin-management-ui-frame"]').attributes('src'))
    expect(startingSrc.searchParams.get('frame')).toBe(firstSrc.searchParams.get('frame'))

    await wrapper.setProps({ plugin: buildPlugin({ state: 'running' }) })
    await flushPromises()
    const reloadedSrc = new URL(wrapper.get('[data-testid="plugin-management-ui-frame"]').attributes('src'))
    expect(reloadedSrc.searchParams.get('frame')).not.toBe(firstSrc.searchParams.get('frame'))
    wrapper.unmount()
  })

  it('binds a one-time MessageChannel and never sends plaintext stored secrets', async () => {
    const wrapper = mountHost()
    await flushPromises()
    const { channel } = await connectBridge(wrapper)

    const init = channel.port1.sent.at(-1) as Record<string, unknown>
    expect(init).toMatchObject({
      version: '3',
      source: 'management_host',
      type: 'host.init',
      payload: {
        plugin: { id: 'example-config-panel', version: '0.2.0' },
        page: { id: 'config', label: '配置页面' },
        config: { default_city: '上海', unit: 'fahrenheit' },
        secrets_configured: { api_token: true, optional_token: false },
        allowed_permissions: [],
      },
    })
    expect(JSON.stringify(init)).not.toContain('secret-value')
    expect((init.payload as Record<string, unknown>)).not.toHaveProperty('secrets')

    channel.port1.emit({
      version: '3', source: 'plugin_management_ui', type: 'secrets.set', request_id: 'set-1',
      payload: { values: { optional_token: 'secret-value' } },
    })
    await flushPromises()
    expect(fetch).toHaveBeenCalledWith('/api/plugins/example-config-panel/secrets', expect.objectContaining({
      method: 'PUT',
      body: JSON.stringify({ values: { optional_token: 'secret-value' } }),
    }))
    expect(channel.port1.sent.at(-1)).toMatchObject({
      type: 'secrets.status.changed',
      request_id: 'set-1',
      payload: { configured: { api_token: true, optional_token: true } },
    })
    expect(JSON.stringify(channel.port1.sent.at(-1))).not.toContain('secret-value')

    channel.port1.emit({
      version: '3', source: 'plugin_management_ui', type: 'secrets.delete', request_id: 'delete-1',
      payload: { keys: ['api_token'] },
    })
    await flushPromises()
    expect(fetch).toHaveBeenCalledWith('/api/plugins/example-config-panel/secrets', expect.objectContaining({
      method: 'DELETE',
      body: JSON.stringify({ keys: ['api_token'] }),
    }))
    wrapper.unmount()
  })

  it('times out stalled initialization after the channel is connected and allows a fresh retry', async () => {
    vi.useFakeTimers()
    let resolveSettings!: (value: never) => void
    vi.mocked(usePluginsStore().fetchSettings).mockImplementationOnce(() => new Promise((resolve) => { resolveSettings = resolve }))
    const wrapper = mountHost()
    await flushPromises()
    const { channel } = await connectBridge(wrapper)
    await vi.advanceTimersByTimeAsync(10_000)
    expect(wrapper.get('[role="alert"]').text()).toContain('预期时间')
    expect(channel.port1.closed).toBe(true)
    resolveSettings({ values: { stale: true } } as never)
    await flushPromises()
    expect(channel.port1.sent).toHaveLength(0)
    await wrapper.get('[role="alert"] button').trigger('click')
    await flushPromises()
    const retry = await connectBridge(wrapper)
    expect(retry.channel.port1.sent.at(-1)).toMatchObject({ type: 'host.init' })
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('ignores unrelated window messages and does not replace an established channel', async () => {
    const wrapper = mountHost()
    await flushPromises()
    const { channel, frameWindow, origin, iframe } = await connectBridge(wrapper)
    dispatchWindowMessage(frameWindow, origin, { type: 'resize', height: 640 })
    dispatchWindowMessage(frameWindow, origin, {
      version: '3', source: 'plugin_management_ui', type: 'page.ready',
      nonce: new URL(iframe.src).searchParams.get('bridge_nonce'),
    })
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(FakeMessageChannel.latest?.port1).toBe(channel.port1)
    wrapper.unmount()
  })

  it('removes the previous frame while resolving a new page and ignores its delayed handshake', async () => {
    const wrapper = mountHost()
    await flushPromises()
    const previous = await connectBridge(wrapper)
    let resolveOrigin!: (digest: ArrayBuffer) => void
    vi.mocked(crypto.subtle.digest).mockImplementationOnce(() => new Promise((resolve) => { resolveOrigin = resolve }))
    await wrapper.setProps({ page: { id: 'secrets', label: '密钥设置' } })
    expect(wrapper.find('[data-testid="plugin-management-ui-frame"]').exists()).toBe(false)
    expect(previous.channel.port1.closed).toBe(true)
    dispatchWindowMessage(previous.frameWindow, previous.origin, {
      version: '3', source: 'plugin_management_ui', type: 'page.ready',
      nonce: new URL(previous.iframe.src).searchParams.get('bridge_nonce'),
    })
    expect(FakeMessageChannel.latest).toBe(previous.channel)
    resolveOrigin(new Uint8Array(32).buffer)
    await flushPromises()
    expect(wrapper.find('[data-testid="plugin-management-ui-frame"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('does not publish stale initialization when the theme changes during a new handshake', async () => {
    const wrapper = mountHost()
    await flushPromises()
    await connectBridge(wrapper)
    let resolveSettings!: (value: never) => void
    vi.mocked(usePluginsStore().fetchSettings).mockImplementationOnce(() => new Promise((resolve) => { resolveSettings = resolve }))
    await wrapper.setProps({ page: { id: 'secrets', label: '密钥设置' } })
    await flushPromises()
    const { channel } = await connectBridge(wrapper)
    useUiShellStore().preferences.themeMode = 'dark'
    await flushPromises()
    expect(channel.port1.sent).toHaveLength(0)
    resolveSettings({ values: { default_city: '杭州' } } as never)
    await flushPromises()
    expect(channel.port1.sent).toHaveLength(1)
    expect(channel.port1.sent[0]).toMatchObject({ type: 'host.init', payload: { config: { default_city: '杭州' }, theme: { mode: 'dark' } } })
    wrapper.unmount()
  })

  it('does not deliver a previous session settings response to a new page', async () => {
    const wrapper = mountHost()
    await flushPromises()
    const previous = await connectBridge(wrapper)
    let resolveSettings!: (value: never) => void
    vi.mocked(usePluginsStore().fetchSettings).mockImplementationOnce(() => new Promise((resolve) => { resolveSettings = resolve }))
    previous.channel.port1.emit({ version: '3', source: 'plugin_management_ui', type: 'settings.reload', request_id: 'old-reload' })
    await wrapper.setProps({ page: { id: 'secrets', label: '密钥设置' } })
    await flushPromises()
    const current = await connectBridge(wrapper)
    resolveSettings({ values: { default_city: '旧页面' } } as never)
    await flushPromises()
    expect(current.channel.port1.sent).toHaveLength(1)
    useUiShellStore().preferences.themeMode = 'dark'
    await flushPromises()
    expect(current.channel.port1.sent.at(-1)).toMatchObject({ type: 'host.init', payload: { config: { default_city: '上海' } } })
    wrapper.unmount()
  })

  it.each(['success', 'failure'])('does not forward a stale management action %s after reconnecting', async (outcome) => {
    const wrapper = mountHost()
    await flushPromises()
    const previous = await connectBridge(wrapper)
    let resolveAction!: (value: Response) => void
    let rejectAction!: (error: Error) => void
    vi.mocked(fetch).mockImplementationOnce(() => new Promise((resolve, reject) => {
      resolveAction = resolve
      rejectAction = reject
    }))
    previous.channel.port1.emit({
      version: '3', source: 'plugin_management_ui', type: 'plugin.action.invoke',
      request_id: 'old-action', payload: { action: 'refresh' },
    })
    await wrapper.setProps({ page: { id: 'secrets', label: '密钥设置' } })
    await flushPromises()
    const current = await connectBridge(wrapper)
    if (outcome === 'success') {
      resolveAction(new Response(JSON.stringify({ plugin_id: 'example-config-panel', action: 'refresh', result: { previous: true } }), { status: 200 }))
    } else {
      rejectAction(new Error('Previous page request failed'))
    }
    await flushPromises()
    expect(current.channel.port1.sent).toHaveLength(1)
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('rejects bridge v1 and keeps long plugin pages inside the visible viewport', async () => {
    const first = mountHost()
    await flushPromises()
    const stale = assignIframeWindow(first)
    dispatchWindowMessage(stale.frameWindow, stale.origin, {
      version: '1', source: 'plugin_management_ui', type: 'page.ready', nonce: new URL(stale.iframe.src).searchParams.get('bridge_nonce'),
    })
    await flushPromises()
    expect(FakeMessageChannel.latest).toBeNull()
    expect(first.find('[data-testid="plugin-management-ui-frame"]').exists()).toBe(false)
    first.unmount()

    FakeMessageChannel.latest = null
    const second = mountHost()
    await flushPromises()
    const { channel, iframe } = await connectBridge(second)
    vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(900)
    vi.spyOn(iframe, 'getBoundingClientRect').mockReturnValue({ top: 180 } as DOMRect)
    channel.port1.emit({
      version: '3', source: 'plugin_management_ui', type: 'ui.resize', payload: { height: 5000 },
    })
    await flushPromises()
    expect(iframe.style.height).toBe('696px')
    channel.port1.emit({
      version: '3', source: 'plugin_management_ui', type: 'ui.resize', payload: { height: 100 },
    })
    await flushPromises()
    expect(iframe.style.height).toBe('320px')
    second.unmount()
  })
})
