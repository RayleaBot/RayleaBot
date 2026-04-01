import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import PluginDetailPage from '@/pages/PluginDetailPage.vue'
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
      display_state: 'running',
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
      command_conflicts: ['weather'],
    }
    pluginsStore.grants = {
      weather: [
        {
          plugin_id: 'weather',
          capability: 'http.request',
          granted_at: '2026-03-22T10:00:00Z',
        },
      ],
    }
    pluginsStore.appendConsole({
      plugin_id: 'weather',
      stream: 'stderr',
      text: 'Traceback (most recent call last): ...',
      timestamp: '2026-03-22T10:00:01Z',
    })

    vi.spyOn(pluginsStore, 'fetchDetail').mockResolvedValue(pluginsStore.current)
    vi.spyOn(pluginsStore, 'fetchGrants').mockResolvedValue(pluginsStore.grants.weather)
    vi.spyOn(socketStore, 'setConsolePlugin').mockImplementation(() => undefined)
    const reconnectSpy = vi.spyOn(socketStore, 'reconnectConsole').mockImplementation(() => undefined)

    socketStore.snapshots.pluginConsole.status = 'reconnecting'
    socketStore.snapshots.pluginConsole.lastError = 'console socket error'

    const wrapper = mount(PluginDetailPage, {
      global: {
        plugins: [ElementPlus, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('当前状态')
    expect(wrapper.text()).toContain('当前授权')
    expect(wrapper.text()).toContain('实时控制台')
    expect(wrapper.text()).toContain('http.request')
    expect(wrapper.text()).toContain('Traceback (most recent call last): ...')
    expect(wrapper.text()).toContain('Weather')
    expect(wrapper.text()).toContain('未验证来源')
    expect(wrapper.text()).toContain('plugins/installed')
    expect(wrapper.text()).toContain('weather')

    const reconnectButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('重新连接'))
    expect(reconnectButton).toBeTruthy()
    await reconnectButton!.trigger('click')

    expect(reconnectSpy).toHaveBeenCalledTimes(1)
  })
})
