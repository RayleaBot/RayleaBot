import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import AppShell from '@/components/AppShell.vue'
import { useSocketStore } from '@/stores/sockets'
import { useSystemStore } from '@/stores/system'

describe('AppShell', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders a chinese shell without the legacy english chrome or duplicate menu button', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        {
          path: '/',
          component: AppShell,
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

    const wrapper = mount(AppShell, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('管理控制台')
    expect(wrapper.text()).toContain('系统状态')
    expect(wrapper.text()).toContain('就绪状态')
    expect(wrapper.text()).toContain('协议中心')
    expect(wrapper.text()).toContain('指令中心')
    expect(wrapper.text()).not.toContain('协议日志')
    expect(wrapper.text()).not.toContain('Management Surface')
    expect(wrapper.text()).not.toContain('Control Plane')
    expect(wrapper.text()).not.toContain('导航')
    expect(wrapper.text()).not.toContain('菜单')
    expect(wrapper.find('.mobile-menu-button').exists()).toBe(false)
  })

  it('expands protocol navigation for the protocol log route', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        {
          path: '/',
          component: AppShell,
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

    const wrapper = mount(AppShell, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.find('.shell-subnav').exists()).toBe(true)
    expect(wrapper.text()).toContain('协议日志')
  })
})
