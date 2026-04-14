import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import PluginDetailPage from '@/views/plugins/PluginDetailView.vue'
import { usePluginsStore } from '@/stores/plugins'
import { useSocketStore } from '@/stores/sockets'

describe('PluginDetailPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders grants and reconnects the console stream', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/plugins/:id', component: PluginDetailPage }],
    })
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
    expect(wrapper.text()).toContain('已注册指令')
    expect(wrapper.text()).toContain('权限与授权')
    expect(wrapper.text()).toContain('实时控制台')
    expect(wrapper.text()).toContain('http.request')
    expect(wrapper.text()).toContain('手动授权')
    expect(wrapper.text()).toContain('查询天气')
    expect(wrapper.text()).toContain('member')
    expect(wrapper.text()).toContain('Traceback (most recent call last): ...')
    expect(wrapper.text()).toContain('plugin weather command weather delivered group message: 杭州晴')
    expect(wrapper.text()).toContain('outbound')
    expect(wrapper.text()).toContain('info')
    expect(wrapper.text()).toContain('Weather')
    expect(wrapper.text()).toContain('未验证来源')
    expect(wrapper.text()).toContain('plugins/installed')
    expect(wrapper.text()).toContain('已识别')
    expect(wrapper.text()).toContain('weather')
    expect(wrapper.find('.console-terminal').exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'PluginCommandsPanel' }).exists()).toBe(true)

    const reconnectButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('重新连接'))
    expect(reconnectButton).toBeTruthy()
    await reconnectButton!.trigger('click')

    expect(historySpy).toHaveBeenCalledWith('weather')
    expect(reconnectSpy).toHaveBeenCalledTimes(1)
  })

  it('escapes unsafe control characters in console output', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/plugins/:id', component: PluginDetailPage }],
    })
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
})
