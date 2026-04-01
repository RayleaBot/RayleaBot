import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it } from 'vitest'

import ConnectionStatusStrip from '@/components/ConnectionStatusStrip.vue'
import { useSocketStore } from '@/stores/sockets'

describe('ConnectionStatusStrip', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders only management channels and suppresses duplicate connection noise', () => {
    const socketStore = useSocketStore()
    socketStore.snapshots.events.status = 'authenticated'
    socketStore.snapshots.tasks.status = 'reconnecting'
    socketStore.snapshots.tasks.lastError = 'tasks 连接异常'
    socketStore.snapshots.logs.status = 'connected'
    socketStore.snapshots.pluginConsole.status = 'auth_failed'

    const wrapper = mount(ConnectionStatusStrip, {
      global: {
        plugins: [ElementPlus],
      },
    })

    expect(wrapper.text()).toContain('事件流')
    expect(wrapper.text()).toContain('已认证')
    expect(wrapper.text()).toContain('任务流')
    expect(wrapper.text()).toContain('重连中')
    expect(wrapper.text()).toContain('日志流')
    expect(wrapper.text()).not.toContain('控制台')
    expect(wrapper.text()).not.toContain('pluginConsole')
    expect(wrapper.text()).not.toContain('tasks 连接异常')
    expect(wrapper.text()).toContain('全部重连')
  })
})
