import Antd from 'ant-design-vue'
import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import VirtualDataViewport from '@/components/VirtualDataViewport.vue'
import PluginManagementUIHost from '@/components/plugins/PluginManagementUIHost.vue'
import PluginDetailPage from '@/views/plugins/PluginDetailView.vue'
import { useConfigStore } from '@/stores/config'
import { usePluginConsoleStore } from '@/stores/plugin-console'
import { usePluginsStore } from '@/stores/plugins'
import { useSocketStore } from '@/stores/sockets'
import type { ConfigDocument } from '@/types/api'

function createFixtureConfig(prefixes: string[]): ConfigDocument {
  return {
    schema_version: '2',
    server: { host: '127.0.0.1', port: 8080 },
    onebot: {
      reverse_ws: { enabled: false, url: '', access_token: '' },
      forward_ws: { enabled: false, url: '', access_token: '' },
      http_api: { enabled: false, url: '', access_token: '' },
      webhook: { enabled: false, url: '', access_token: '' },
    },
    database: { engine: 'sqlite', path: 'data/rayleabot.db' },
    command: { prefixes },
    builtin_features: {
      menu: {
        commands: ['help', '帮助'],
        prefixes: [],
      },
    },
    admin: {
      super_admins: [],
      session_ttl_days: 7,
      sliding_renewal: true,
      max_sessions: 3,
      login_fail_limit: 5,
      login_fail_window_seconds: 300,
    },
    permission: {
      default_level: 'everyone',
    },
    render: {
      worker_count: 1,
      browser_args: ['--disable-gpu'],
      browser_path: '',
      timeout_seconds: 30,
      queue_wait_timeout_seconds: 15,
      queue_max_length: 32,
      footer_template: 'Created By RayleaBot {{rayleabot_version}} & Plugin {{plugin_name}} {{plugin_version}}',
    },
    scheduler: { timezone: '' },
    runtime: {
      plugin_init_timeout_seconds: 30,
      plugin_init_max_total_seconds: 300,
      plugin_event_timeout_seconds: 60,
      max_pending_events_per_plugin: 16,
      max_pending_control_events_per_plugin: 4,
      nodejs_max_old_space_size_mb: 256,
      dependency_install_timeout_seconds: 900,
      max_concurrent_dependency_installs: 1,
      ipc_pending_actions_max: 256,
      ipc_action_burst_limit: '100/1s',
      stderr_rate_limit_bytes_per_second: 262144,
      max_concurrent_tasks_per_plugin: 4,
      crash_backoff_initial_seconds: 2,
      crash_backoff_max_seconds: 60,
      shutdown_grace_seconds: 10,
      ipc_message_max_bytes: 8388608,
    },
    storage: { kv_value_max_bytes: 65536, kv_total_limit_mb: 16, file_max_bytes: 10485760, plugin_workdir_soft_limit_mb: 256 },
    data: {
      audit_logs_retention_days: 90,
      event_records_retention_days: 7,
      download_cache_retention_days: 15,
    },
    log: { level: 'info', retention_days: 7, rate_limit_per_plugin: '200/10s' },
    message: {
      rate_limit_per_plugin: '20/10s',
      rate_limit_per_target: '5/5s',
      circuit_breaker_seconds: 30,
    },
    user: { command_rate_limit: '10/60s', cooldown_reply: true },
    group: { command_rate_limit: '30/60s' },
    adapter: {
      connect_timeout_seconds: 15,
      reconnect_initial_seconds: 2,
      reconnect_multiplier: 2,
      reconnect_max_seconds: 120,
      reconnect_jitter_ratio: 0.2,
    },
    http: { timeout_seconds: 10, max_retries: 2, allow_private_hosts: [] },
    web: { exposure_mode: 'localhost_only', setup_local_only: true },
    backup: { default_consistency: 'offline' },
  }
}

function mockScrollerMetrics(wrapper: ReturnType<typeof mount>, clientHeight: number) {
  const scroller = wrapper.get('.plugin-console-panel .data-viewport__scroller').element as HTMLElement
  let internalScrollTop = 0

  Object.defineProperty(scroller, 'clientHeight', {
    configurable: true,
    value: clientHeight,
  })
  Object.defineProperty(scroller, 'scrollTop', {
    configurable: true,
    get: () => internalScrollTop,
    set: (value: number) => {
      internalScrollTop = Math.floor(value)
    },
  })
  Object.defineProperty(scroller, 'scrollHeight', {
    configurable: true,
    get: () => {
      const style = wrapper.get('.plugin-console-panel .data-viewport__canvas').attributes('style')
      const matched = /height:\s*(\d+)px/.exec(style)
      return matched ? Number(matched[1]) : 0
    },
  })

  return scroller
}

function getViewportMetrics(wrapper: ReturnType<typeof mount>) {
  return wrapper.findComponent(VirtualDataViewport).vm.getScrollMetrics()
}

async function openConsoleTab(wrapper: ReturnType<typeof mount>) {
  const consoleTab = wrapper.findAll('[role="tab"]').find((candidate) => candidate.text().includes('实时控制台'))
  expect(consoleTab).toBeTruthy()
  await consoleTab!.trigger('click')
  await nextTick()
  await flushPromises()
}

async function waitForConsoleBottomSync() {
  for (let attempt = 0; attempt < 5; attempt += 1) {
    await nextTick()
    await flushPromises()
  }
}

describe('PluginDetailPage', () => {
  function createPluginRouter() {
    return createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/plugins/:id', name: 'plugin-detail', component: PluginDetailPage },
        { path: '/commands', name: 'commands', component: { template: '<div>commands</div>' } },
        { path: '/logs/history', name: 'logs-history', component: { template: '<div>logs-history</div>' } },
      ],
    })
  }

  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders manifest metadata and reconnects the console stream', async () => {
    const router = createPluginRouter()
    await router.push('/plugins/weather')
    await router.isReady()

    const pluginsStore = usePluginsStore()
    const pluginConsoleStore = usePluginConsoleStore()
    const configStore = useConfigStore()
    const socketStore = useSocketStore()

    pluginsStore.current = {
      id: 'weather',
      name: 'Weather',
      role: 'user',
        state: 'running',
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
      capability_parameters: {
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
          name: '我的运势',
          aliases: ['今日运势'],
          description: '查看今日运势',
          usage: '我的运势',
          permission: 'everyone',
          command_source: 'dynamic',
          declaration_id: 'fortune',
        },
      ],
      command_conflicts: [],
    }
    pluginConsoleStore.appendConsole({
      plugin_id: 'weather',
      stream: 'stdout',
      text: 'worker ready',
      timestamp: '2026-03-22T10:00:00Z',
    })
    pluginConsoleStore.appendConsole({
      plugin_id: 'weather',
      stream: 'stderr',
      text: 'Traceback (most recent call last): ...',
      timestamp: '2026-03-22T10:00:01Z',
    })
    pluginConsoleStore.appendConsole({
      plugin_id: 'weather',
      stream: 'system',
      text: 'process heartbeat ok',
      timestamp: '2026-03-22T10:00:01.500Z',
    })
    pluginConsoleStore.appendOutboundLog({
      log_id: 'log_weather_outbound_0001',
      timestamp: '2026-03-22T10:00:02Z',
      level: 'info',
      protocol: 'onebot11',
      source: 'adapter.onebot11',
      plugin_id: 'weather',
      request_id: 'req_runtime_delivery_0001',
      message: 'plugin weather command weather delivered group message: 杭州晴',
    })

    configStore.document = createFixtureConfig(['#'])
    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(pluginsStore, 'fetchDetail').mockResolvedValue(pluginsStore.current)
    const historySpy = vi.spyOn(pluginConsoleStore, 'fetchOutboundConsoleHistory').mockResolvedValue([])
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

    expect(wrapper.text()).toContain('运行摘要')
    const tabLabels = wrapper.findAll('[role="tab"]').map((tab) => tab.text())
    expect(tabLabels[0]).toContain('运行摘要')
    expect(tabLabels[1]).toContain('插件指令')
    expect(tabLabels[2]).toContain('实时控制台')
    expect(wrapper.get('.plugin-detail-summary-panel').attributes('aria-label')).toBe('运行摘要')
    expect(wrapper.find('.plugin-detail-side-column').exists()).toBe(false)
    expect(wrapper.text()).toContain('包信息')
    expect(wrapper.text()).toContain('来源信息')
    expect(wrapper.text()).toContain('Manifest 元数据')
    expect(wrapper.text()).toContain('详细信息')
    expect(wrapper.text()).toContain('运行配置')
    expect(wrapper.text()).toContain('插件指令')
    expect(wrapper.text()).toContain('动态指令')
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
    expect(wrapper.text()).toContain('发起 HTTP 请求')
    expect(wrapper.text()).toContain('写入插件日志')
    expect(wrapper.text()).toContain('生成渲染图片')
    expect(wrapper.find('[title="原始能力：http.request"]').exists()).toBe(true)
    expect(wrapper.find('[title="原始能力：logger.write"]').exists()).toBe(true)
    expect(wrapper.find('[title="原始能力：render.image"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('查看今日运势')
    expect(wrapper.text()).toContain('所有成员')
    expect(wrapper.text()).toContain('#我的运势')
    expect(wrapper.text()).toContain('今日运势')
    expect(wrapper.text()).toContain('4 条输出')
    expect(wrapper.text()).toContain('worker ready')
    expect(wrapper.text()).toContain('Traceback (most recent call last): ...')
    expect(wrapper.text()).toContain('process heartbeat ok')
    expect(wrapper.text()).toContain('plugin weather command weather delivered group message: 杭州晴')
    expect(wrapper.text()).toContain('标准输出')
    expect(wrapper.text()).toContain('错误输出')
    expect(wrapper.text()).toContain('系统')
    expect(wrapper.text()).toContain('外发')
    expect(wrapper.text()).toContain('信息')
    expect(wrapper.text()).toContain('当前插件指令')
    expect(wrapper.text()).toContain('当前插件日志')
    expect(wrapper.text()).toContain('Weather')
    expect(wrapper.text()).toContain('未验证来源')
    expect(wrapper.text()).toContain('plugins/installed')
    expect(wrapper.text()).toContain('运行中')
    expect(wrapper.text()).toContain('我的运势')
    expect(wrapper.text()).not.toContain('fortune')
    expect(wrapper.find('.console-terminal').exists()).toBe(true)
    expect(wrapper.findComponent(VirtualDataViewport).exists()).toBe(true)
    expect(wrapper.findComponent(VirtualDataViewport).props('dynamicItemHeight')).toBe(true)
    expect(wrapper.findComponent(VirtualDataViewport).props('itemHeight')).toBe(84)
    expect(wrapper.findComponent(VirtualDataViewport).props('overscan')).toBe(6)
    expect(wrapper.findComponent(VirtualDataViewport).props('viewportHeight')).toBe('max(420px, calc(100vh - 430px))')
    expect(wrapper.findAll('.console-terminal-line')).toHaveLength(4)
    expect(wrapper.findAll('.plugin-holo-button')).toHaveLength(1)
    expect(wrapper.findComponent({ name: 'PluginCommandsPanel' }).exists()).toBe(true)
    expect(wrapper.find('[role="tab"][aria-selected="true"]').text()).toContain('运行摘要')
    mockScrollerMetrics(wrapper, 346)
    await openConsoleTab(wrapper)
    expect(wrapper.find('.app-page').classes()).toContain('app-page--full-height')
    expect(wrapper.find('.plugin-detail-tab-card').classes()).toContain('is-console-tab-active')
    await waitForConsoleBottomSync()
    expect(getViewportMetrics(wrapper).scrollTop).toBeGreaterThanOrEqual(
      getViewportMetrics(wrapper).scrollHeight - getViewportMetrics(wrapper).clientHeight - 1,
    )

    const reconnectButton = wrapper.findAll('button').find((candidate) => candidate.attributes('aria-label') === '重新连接')
    expect(reconnectButton).toBeTruthy()
    await reconnectButton!.trigger('click')

    expect(historySpy).toHaveBeenCalledWith('weather')
    expect(reconnectSpy).toHaveBeenCalledTimes(1)
  })

  it('keeps the console anchored to the bottom after the page transition settles', async () => {
    const router = createPluginRouter()
    await router.push('/plugins/weather')
    await router.isReady()

    const pluginsStore = usePluginsStore()
    const pluginConsoleStore = usePluginConsoleStore()
    const socketStore = useSocketStore()

    pluginsStore.current = {
      id: 'weather',
      name: 'Weather',
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
      commands: [],
      command_conflicts: [],
    }
    pluginConsoleStore.appendConsole({
      plugin_id: 'weather',
      stream: 'stdout',
      text: 'worker ready',
      timestamp: '2026-03-22T10:00:00Z',
    })

    vi.spyOn(pluginsStore, 'fetchDetail').mockResolvedValue(pluginsStore.current)
    vi.spyOn(pluginConsoleStore, 'fetchOutboundConsoleHistory').mockResolvedValue([])
    vi.spyOn(socketStore, 'setConsolePlugin').mockImplementation(() => undefined)

    const wrapper = mount(PluginDetailPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()
    expect(wrapper.findComponent(VirtualDataViewport).exists()).toBe(true)
    expect(wrapper.find('.console-terminal-line').text()).toContain('worker ready')
    await openConsoleTab(wrapper)
    expect(wrapper.findComponent(VirtualDataViewport).vm.isAtBottom()).toBe(true)
  })

  it('pauses bottom follow after the user scrolls away from the latest row', async () => {
    const router = createPluginRouter()
    await router.push('/plugins/weather')
    await router.isReady()

    const pluginsStore = usePluginsStore()
    const pluginConsoleStore = usePluginConsoleStore()
    const socketStore = useSocketStore()

    pluginsStore.current = {
      id: 'weather',
      name: 'Weather',
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
      commands: [],
      command_conflicts: [],
    }

    pluginConsoleStore.appendConsole({
      plugin_id: 'weather',
      stream: 'stdout',
      text: 'worker ready',
      timestamp: '2026-03-22T10:00:00Z',
    })
    for (let index = 1; index <= 12; index += 1) {
      pluginConsoleStore.appendConsole({
        plugin_id: 'weather',
        stream: index % 2 === 0 ? 'stderr' : 'system',
        text: `trace line ${index}`,
        timestamp: `2026-03-22T10:00:${String(index).padStart(2, '0')}Z`,
      })
    }

    vi.spyOn(pluginsStore, 'fetchDetail').mockResolvedValue(pluginsStore.current)
    vi.spyOn(pluginConsoleStore, 'fetchOutboundConsoleHistory').mockResolvedValue([])
    vi.spyOn(socketStore, 'setConsolePlugin').mockImplementation(() => undefined)

    const wrapper = mount(PluginDetailPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()
    const scroller = mockScrollerMetrics(wrapper, 346)
    await openConsoleTab(wrapper)
    await waitForConsoleBottomSync()
    expect(getViewportMetrics(wrapper).scrollTop).toBeGreaterThan(0)

    scroller.scrollTop = 0
    await wrapper.get('.plugin-console-panel .data-viewport__scroller').trigger('scroll')
    await nextTick()
    await flushPromises()

    expect(getViewportMetrics(wrapper).scrollTop).toBe(0)

    pluginConsoleStore.appendConsole({
      plugin_id: 'weather',
      stream: 'system',
      text: 'new line after scroll',
      timestamp: '2026-03-22T10:00:02Z',
    })
    await nextTick()
    await flushPromises()

    expect(getViewportMetrics(wrapper).scrollTop).toBe(0)
  })

  it('escapes unsafe control characters in console output', async () => {
    const router = createPluginRouter()
    await router.push('/plugins/weather')
    await router.isReady()

    const pluginsStore = usePluginsStore()
    const pluginConsoleStore = usePluginConsoleStore()
    const socketStore = useSocketStore()

    pluginsStore.current = {
      id: 'weather',
      name: 'Weather',
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
      commands: [],
      command_conflicts: [],
    }
    pluginConsoleStore.appendOutboundLog({
      log_id: 'log_weather_outbound_unsafe_0001',
      timestamp: '2026-03-22T10:00:02Z',
      level: 'info',
      protocol: 'onebot11',
      source: 'adapter.onebot11',
      plugin_id: 'weather',
      request_id: 'req_runtime_delivery_unsafe_0001',
      message: 'plugin weather command weather delivered group message: 测试群名片\u2066~喵',
    })

    vi.spyOn(pluginsStore, 'fetchDetail').mockResolvedValue(pluginsStore.current)
    vi.spyOn(pluginConsoleStore, 'fetchOutboundConsoleHistory').mockResolvedValue([])
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
    const pluginConsoleStore = usePluginConsoleStore()
    const socketStore = useSocketStore()

    let resolveDetail: (() => void) | null = null

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(pluginsStore, 'fetchDetail').mockImplementation(() => (
      new Promise((resolve) => {
        resolveDetail = () => resolve({
          id: 'weather',
          name: 'Weather',
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
          commands: [],
          command_conflicts: [],
        })
      })
    ))
    vi.spyOn(pluginConsoleStore, 'fetchOutboundConsoleHistory').mockResolvedValue([])
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

  it('shows the management panel for plugins that expose a management_ui entry', async () => {
    const router = createPluginRouter()
    await router.push('/plugins/example-config-panel?panel=management-ui')
    await router.isReady()

    const configStore = useConfigStore()
    const pluginsStore = usePluginsStore()
    const pluginConsoleStore = usePluginConsoleStore()
    const socketStore = useSocketStore()

    const detail = {
      id: 'example-config-panel',
      name: 'Example Config Panel',
      role: 'example',
        state: 'disabled',
      version: '0.1.0',
      runtime: 'python',
      type: 'managed_runtime',
      entry: 'main.py',
      description: 'Python example plugin demonstrating config.read and config.write.',
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
          {
            id: 'secrets',
            label: '密钥设置',
            entry: 'web/secrets.html',
          },
        ],
      },
      commands: [],
      command_conflicts: [],
      declared_capabilities: ['config.read', 'config.write'],
    } as const

    pluginsStore.current = detail

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(pluginsStore, 'fetchDetail').mockResolvedValue(detail)
    vi.spyOn(pluginConsoleStore, 'fetchOutboundConsoleHistory').mockResolvedValue([])
    vi.spyOn(socketStore, 'setConsolePlugin').mockImplementation(() => undefined)
    vi.spyOn(pluginsStore, 'fetchSettings').mockResolvedValue({
      plugin_id: 'example-config-panel',
      values: {
        default_city: '上海',
        unit: 'fahrenheit',
      },
    })

    const wrapper = mount(PluginDetailPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('概览')
    expect(wrapper.text()).toContain('配置页面')
    expect(wrapper.text()).toContain('密钥设置')
    expect(wrapper.find('.app-page').classes()).toContain('app-page--full-height')
    expect(wrapper.find('.plugin-detail-panel-switch').exists()).toBe(true)
    expect(wrapper.find('[data-testid="plugin-management-ui-host"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="plugin-management-ui-frame"]').attributes('src')).toContain('/plugin-ui/example-config-panel/web/index.html')
    expect(wrapper.find('.console-terminal').exists()).toBe(false)

    await router.push('/plugins/example-config-panel?panel=management-ui&management_page=secrets')
    await flushPromises()

    expect(wrapper.get('[data-testid="plugin-management-ui-frame"]').attributes('src')).toContain('/plugin-ui/example-config-panel/web/secrets.html')
    expect(wrapper.find('.app-page').classes()).toContain('app-page--full-height')

    await router.push('/plugins/example-config-panel')
    await flushPromises()

    expect(wrapper.find('.app-page').classes()).not.toContain('app-page--full-height')
  })

  it('normalizes single-page management plugins to the declared management page query', async () => {
    const router = createPluginRouter()
    await router.push('/plugins/example-config-panel?panel=management-ui')
    await router.isReady()

    const configStore = useConfigStore()
    const pluginsStore = usePluginsStore()
    const pluginConsoleStore = usePluginConsoleStore()
    const socketStore = useSocketStore()

    const detail = {
      id: 'example-config-panel',
      name: 'Example Config Panel',
      role: 'example',
        state: 'disabled',
      version: '0.1.0',
      runtime: 'python',
      type: 'managed_runtime',
      entry: 'main.py',
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
      declared_capabilities: ['config.read'],
    } as const

    pluginsStore.current = detail

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(pluginsStore, 'fetchDetail').mockResolvedValue(detail)
    vi.spyOn(pluginConsoleStore, 'fetchOutboundConsoleHistory').mockResolvedValue([])
    vi.spyOn(socketStore, 'setConsolePlugin').mockImplementation(() => undefined)

    const wrapper = mount(PluginDetailPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('概览')
    expect(wrapper.text()).toContain('配置页面')
    expect(wrapper.find('.plugin-detail-panel-switch').exists()).toBe(true)
    expect(wrapper.find('[data-testid="plugin-management-ui-host"]').exists()).toBe(true)
    expect(wrapper.getComponent(PluginManagementUIHost).props('page')).toEqual({
      id: 'config',
      label: '配置页面',
      entry: 'web/index.html',
    })
    expect(wrapper.get('[data-testid="plugin-management-ui-frame"]').attributes('src')).toContain('/plugin-ui/example-config-panel/web/index.html')
    expect(router.currentRoute.value.query).toEqual({ panel: 'management-ui', management_page: 'config' })
  })
})
