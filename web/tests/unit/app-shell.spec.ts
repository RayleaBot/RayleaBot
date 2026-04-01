import ElementPlus from 'element-plus'
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
            { path: 'tasks', component: { template: '<div>任务</div>' } },
            { path: 'logs', component: { template: '<div>日志</div>' } },
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
        plugins: [ElementPlus, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('管理控制台')
    expect(wrapper.text()).toContain('系统状态')
    expect(wrapper.text()).toContain('就绪状态')
    expect(wrapper.text()).not.toContain('Management Surface')
    expect(wrapper.text()).not.toContain('Control Plane')
    expect(wrapper.text()).not.toContain('导航')
    expect(wrapper.text()).not.toContain('菜单')
    expect(wrapper.find('.mobile-menu-button').exists()).toBe(false)
  })
})
