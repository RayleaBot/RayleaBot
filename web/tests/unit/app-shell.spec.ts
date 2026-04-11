import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import BasicLayout from '@/layouts/BasicLayout.vue'
import { useSocketStore } from '@/stores/sockets'
import { useSystemStore } from '@/stores/system'
import { useUiShellStore } from '@/stores/ui-shell'

describe('BasicLayout', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders a compact shell header with theme-aware sider styling', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        {
          path: '/',
          component: BasicLayout,
          children: [
            {
              path: '',
              component: { template: '<div>内容</div>' },
              meta: { title: '系统状态' },
            },
            { path: 'plugins', component: { template: '<div>插件</div>' } },
            { path: 'commands', component: { template: '<div>指令中心</div>' } },
            { path: 'tasks', component: { template: '<div>任务</div>' } },
            { path: 'logs', component: { template: '<div>日志</div>' } },
            { path: 'protocols', component: { template: '<div>协议中心</div>' } },
            { path: 'protocols/logs', component: { template: '<div>协议日志</div>' } },
            { path: 'config', component: { template: '<div>配置</div>' } },
          ],
        },
      ],
    })
    await router.push('/')
    await router.isReady()

    const systemStore = useSystemStore()
    systemStore.system = {
      status: 'running',
      adapter_state: 'connected',
      active_plugins: 1,
      uptime_seconds: 12,
    }
    systemStore.readiness = {
      status: 'ready',
    }

    const socketStore = useSocketStore()
    socketStore.snapshots.events.status = 'authenticated'
    socketStore.snapshots.tasks.status = 'authenticated'
    socketStore.snapshots.logs.status = 'authenticated'
    socketStore.snapshots.pluginConsole.status = 'disconnected'
    const uiShellStore = useUiShellStore()
    uiShellStore.setThemeMode('light')

    const wrapper = mount(BasicLayout, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.get('[data-testid="app-sider"]').classes()).toContain('ant-layout-sider-light')
    expect(wrapper.get('[data-testid="theme-toggle"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="header-search"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="app-header"]').text()).not.toContain('事件流')
    expect(wrapper.get('[data-testid="app-header"]').text()).not.toContain('保持正式契约')
    expect(wrapper.text()).toContain('系统状态')
    expect(wrapper.text()).toContain('协议中心')
    expect(wrapper.text()).toContain('指令中心')
    expect(wrapper.text()).toContain('运维')
    expect(wrapper.text()).toContain('协议')
    expect(wrapper.text()).toContain('系统')
    expect(wrapper.text()).not.toContain('管理控制台')
    expect(wrapper.text()).not.toContain('Management Surface')
    expect(wrapper.text()).not.toContain('Control Plane')
  })

  it('expands protocol navigation for the protocol log route', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        {
          path: '/',
          component: BasicLayout,
          children: [
            { path: '', component: { template: '<div>内容</div>' } },
            { path: 'protocols', component: { template: '<div>协议中心</div>' } },
            { path: 'protocols/logs', component: { template: '<div>协议日志</div>' }, meta: { title: '协议日志' } },
          ],
        },
      ],
    })
    await router.push('/protocols/logs')
    await router.isReady()

    const systemStore = useSystemStore()
    systemStore.system = {
      status: 'running',
      adapter_state: 'connected',
      active_plugins: 1,
      uptime_seconds: 12,
    }
    systemStore.readiness = {
      status: 'ready',
    }

    const wrapper = mount(BasicLayout, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.find('.ant-menu-submenu-open').exists()).toBe(true)
    expect(wrapper.text()).toContain('协议日志')
  })
})
