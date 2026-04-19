import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { ApiError } from '@/lib/http'
import PluginDetailPage from '@/views/plugins/PluginDetailView.vue'
import { useConfigStore } from '@/stores/config'
import { usePluginsStore } from '@/stores/plugins'
import { useSocketStore } from '@/stores/sockets'

describe('PluginDetailPage', () => {
  function createPluginRouter() {
    return createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/plugins/:id', name: 'plugin-detail', component: PluginDetailPage },
        { path: '/commands', name: 'commands', component: { template: '<div>commands</div>' } },
        { path: '/logs/history', name: 'logs-history', component: { template: '<div>logs-history</div>' } },
        { path: '/tasks', name: 'tasks', component: { template: '<div>tasks</div>' } },
      ],
    })
  }

  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders grants and reconnects the console stream', async () => {
    const router = createPluginRouter()
    await router.push('/plugins/weather')
    await router.isReady()

    const pluginsStore = usePluginsStore()
    const socketStore = useSocketStore()

    pluginsStore.current = {
      id: 'weather',
      name: 'Weather',
      role: 'user',
      registration_state: 'installed',
      desired_state: 'enabled',
      runtime_state: 'running',
      display_state: 'discovered',
      version: '1.4.2',
      runtime: 'python',
      type: 'managed_runtime',
      entry: 'plugin.py',
      description: '提供当前城市天气与未来天气查询。',
      author: 'raylea',
      license: 'MIT',
      sdk_min_version: '1.2.0',
      runtime_version: '>=3.12',
      min_core_version: '0.2.0',
      data_schema_version: 'weather-v2',
      concurrency: 3,
      platforms: ['windows-x64', 'linux-x64'],
      default_config: {
        unit: 'metric',
        forecast_days: 3,
      },
      declared_capabilities: ['http.request', 'logger.write', 'render.image'],
      dependencies: {
        python: ['httpx==0.28.1'],
      },
      scopes: {
        http_hosts: ['api.weather.example'],
        storage_roots: ['plugin_data'],
      },
      icon: 'assets/weather.svg',
      repo: 'https://github.com/RayleaBot/plugins-weather',
      homepage: 'https://plugins.rayleabot.local/weather',
      keywords: ['weather', 'forecast', 'climate'],
      screenshots: [
        {
          path: 'assets/overview.svg',
          alt: '天气总览卡片',
        },
      ],
      system_dependencies: ['OneBot11 connection'],
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
      permissions: [
        {
          capability: 'http.request',
          requirement: 'required',
          status: 'granted',
          source: 'persisted',
          expires_at: null,
        },
      ],
    }
    pluginsStore.grants = {
      weather: [
        {
          plugin_id: 'weather',
          capability: 'http.request',
          granted_at: '2026-03-22T10:00:00Z',
          source: 'persisted',
          expires_at: null,
        },
      ],
    }
    pluginsStore.appendConsole({
      plugin_id: 'weather',
      stream: 'stderr',
      text: 'Traceback (most recent call last): ...',
      timestamp: '2026-03-22T10:00:01Z',
    })
    pluginsStore.appendOutboundLog({
      log_id: 'log_weather_outbound_0001',
      timestamp: '2026-03-22T10:00:02Z',
      level: 'info',
      protocol: 'onebot11',
      source: 'adapter.onebot11',
      plugin_id: 'weather',
      request_id: 'req_runtime_delivery_0001',
      message: 'plugin weather command weather delivered group message: 杭州晴',
    })

    vi.spyOn(pluginsStore, 'fetchDetail').mockResolvedValue(pluginsStore.current)
    vi.spyOn(pluginsStore, 'fetchGrants').mockResolvedValue(pluginsStore.grants.weather)
    const historySpy = vi.spyOn(pluginsStore, 'fetchOutboundConsoleHistory').mockResolvedValue([])
    vi.spyOn(socketStore, 'setConsolePlugin').mockImplementation(() => undefined)
    const reconnectSpy = vi.spyOn(socketStore, 'reconnectConsole').mockImplementation(() => undefined)

    socketStore.snapshots.pluginConsole.status = 'reconnecting'
    socketStore.snapshots.pluginConsole.lastError = 'console socket error'

    const wrapper = mount(PluginDetailPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('当前状态')
    expect(wrapper.text()).toContain('包与运行信息')
    expect(wrapper.text()).toContain('Manifest 元数据')
    expect(wrapper.text()).toContain('运行配置')
    expect(wrapper.text()).toContain('已注册指令')
    expect(wrapper.text()).toContain('权限与授权')
    expect(wrapper.text()).toContain('实时控制台')
    expect(wrapper.text()).toContain('1.4.2')
    expect(wrapper.text()).toContain('managed_runtime')
    expect(wrapper.text()).toContain('plugin.py')
    expect(wrapper.text()).toContain('raylea')
    expect(wrapper.text()).toContain('MIT')
    expect(wrapper.text()).toContain('assets/weather.svg')
    expect(wrapper.text()).toContain('https://github.com/RayleaBot/plugins-weather')
    expect(wrapper.text()).toContain('https://plugins.rayleabot.local/weather')
    expect(wrapper.text()).toContain('weather-v2')
    expect(wrapper.text()).toContain('httpx==0.28.1')
    expect(wrapper.text()).toContain('api.weather.example')
    expect(wrapper.text()).toContain('forecast_days')
    expect(wrapper.text()).toContain('assets/overview.svg')
    expect(wrapper.text()).toContain('天气总览卡片')
    expect(wrapper.text()).toContain('http.request')
    expect(wrapper.text()).toContain('手动授权')
    expect(wrapper.text()).toContain('查询天气')
    expect(wrapper.text()).toContain('member')
    expect(wrapper.text()).toContain('Traceback (most recent call last): ...')
    expect(wrapper.text()).toContain('plugin weather command weather delivered group message: 杭州晴')
    expect(wrapper.text()).toContain('outbound')
    expect(wrapper.text()).toContain('info')
    expect(wrapper.text()).toContain('当前插件指令')
    expect(wrapper.text()).toContain('当前插件日志')
    expect(wrapper.text()).toContain('Weather')
    expect(wrapper.text()).toContain('未验证来源')
    expect(wrapper.text()).toContain('plugins/installed')
    expect(wrapper.text()).toContain('已识别')
    expect(wrapper.text()).toContain('weather')
    expect(wrapper.find('.console-terminal').exists()).toBe(true)
    expect(wrapper.findAll('.plugin-holo-button')).toHaveLength(1)
    expect(wrapper.findComponent({ name: 'PluginCommandsPanel' }).exists()).toBe(true)

    const reconnectButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('重新连接'))
    expect(reconnectButton).toBeTruthy()
    await reconnectButton!.trigger('click')

    expect(historySpy).toHaveBeenCalledWith('weather')
    expect(reconnectSpy).toHaveBeenCalledTimes(1)
  })

  it('reconfirms persisted grants when enabling requires scope review', async () => {
    const router = createPluginRouter()
    await router.push('/plugins/weather')
    await router.isReady()

    const pluginsStore = usePluginsStore()
    const socketStore = useSocketStore()

    pluginsStore.current = {
      id: 'weather',
      name: 'Weather',
      role: 'user',
      registration_state: 'installed',
      desired_state: 'disabled',
      runtime_state: 'stopped',
      display_state: 'disabled',
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
      commands: [],
      command_conflicts: [],
      permissions: [
        {
          capability: 'http.request',
          requirement: 'required',
          status: 'granted',
          source: 'persisted',
          expires_at: null,
        },
        {
          capability: 'logger.write',
          requirement: 'optional',
          status: 'granted',
          source: 'config_auto',
          expires_at: null,
        },
      ],
    }
    pluginsStore.grants = {
      weather: [
        {
          plugin_id: 'weather',
          capability: 'http.request',
          granted_at: '2026-03-22T10:00:00Z',
          source: 'persisted',
          expires_at: null,
        },
      ],
    }

    vi.spyOn(pluginsStore, 'fetchDetail').mockResolvedValue(pluginsStore.current)
    vi.spyOn(pluginsStore, 'fetchGrants').mockResolvedValue(pluginsStore.grants.weather)
    vi.spyOn(pluginsStore, 'fetchOutboundConsoleHistory').mockResolvedValue([])
    vi.spyOn(socketStore, 'setConsolePlugin').mockImplementation(() => undefined)
    vi.spyOn(socketStore, 'reconnectConsole').mockImplementation(() => undefined)

    const executeActionSpy = vi.spyOn(pluginsStore, 'executeAction')
      .mockRejectedValueOnce(new ApiError(
        '插件所需能力尚未获批',
        409,
        'plugin.permission_pending',
        'req_permission_pending_scope_changed',
        {
          plugin_id: 'weather',
          scope_changed: true,
        },
        'errors.plugin.permission_pending',
      ))
      .mockResolvedValueOnce({
        ...pluginsStore.current,
        desired_state: 'enabled',
        runtime_state: 'starting',
        display_state: 'enabling',
      })
    const grantCapabilitySpy = vi.spyOn(pluginsStore, 'grantCapability').mockResolvedValue({
      plugin_id: 'weather',
      capability: 'http.request',
      granted_at: '2026-03-22T10:05:00Z',
      source: 'persisted',
      expires_at: null,
    })

    const wrapper = mount(PluginDetailPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    await wrapper.find('.plugin-holo-button').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('重新确认插件权限')
    expect(wrapper.text()).toContain('作用域发生变化')
    expect(wrapper.text()).toContain('http.request')
    expect(wrapper.text()).not.toContain('当前未声明权限')

    const confirmButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('重新确认选中项'))
    expect(confirmButton).toBeTruthy()
    await confirmButton!.trigger('click')
    await flushPromises()

    expect(grantCapabilitySpy).toHaveBeenCalledWith('weather', { capability: 'http.request' })
    expect(executeActionSpy).toHaveBeenCalledTimes(2)
  })

  it('escapes unsafe control characters in console output', async () => {
    const router = createPluginRouter()
    await router.push('/plugins/weather')
    await router.isReady()

    const pluginsStore = usePluginsStore()
    const socketStore = useSocketStore()

    pluginsStore.current = {
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
      commands: [],
      command_conflicts: [],
      permissions: [],
    }
    pluginsStore.appendOutboundLog({
      log_id: 'log_weather_outbound_unsafe_0001',
      timestamp: '2026-03-22T10:00:02Z',
      level: 'info',
      protocol: 'onebot11',
      source: 'adapter.onebot11',
      plugin_id: 'weather',
      request_id: 'req_runtime_delivery_unsafe_0001',
      message: 'plugin weather command weather delivered group message: 群星怒\u2066~喵',
    })

    vi.spyOn(pluginsStore, 'fetchDetail').mockResolvedValue(pluginsStore.current)
    vi.spyOn(pluginsStore, 'fetchGrants').mockResolvedValue([])
    vi.spyOn(pluginsStore, 'fetchOutboundConsoleHistory').mockResolvedValue([])
    vi.spyOn(socketStore, 'setConsolePlugin').mockImplementation(() => undefined)
    vi.spyOn(socketStore, 'reconnectConsole').mockImplementation(() => undefined)

    const wrapper = mount(PluginDetailPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    const consoleText = wrapper.find('.console-terminal-line pre').text()
    expect(consoleText).toContain('\\u2066')
    expect(consoleText).not.toContain('\u2066')
  })

  it('keeps the console socket closed when an old detail request resolves after unmount', async () => {
    const router = createPluginRouter()
    await router.push('/plugins/weather')
    await router.isReady()

    const configStore = useConfigStore()
    const pluginsStore = usePluginsStore()
    const socketStore = useSocketStore()

    let resolveDetail: (() => void) | null = null

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(pluginsStore, 'fetchDetail').mockImplementation(() => (
      new Promise((resolve) => {
        resolveDetail = () => resolve({
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
          commands: [],
          command_conflicts: [],
          permissions: [],
        })
      })
    ))
    vi.spyOn(pluginsStore, 'fetchGrants').mockResolvedValue([])
    vi.spyOn(pluginsStore, 'fetchOutboundConsoleHistory').mockResolvedValue([])
    const setConsolePluginSpy = vi.spyOn(socketStore, 'setConsolePlugin').mockImplementation(() => undefined)

    const wrapper = mount(PluginDetailPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()
    wrapper.unmount()

    expect(setConsolePluginSpy).toHaveBeenLastCalledWith(null)

    setConsolePluginSpy.mockClear()
    resolveDetail?.()
    await flushPromises()

    expect(setConsolePluginSpy).not.toHaveBeenCalled()
  })
})
