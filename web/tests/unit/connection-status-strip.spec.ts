import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it } from 'vitest'

import ConnectionStatusStrip from '@/components/ConnectionStatusStrip.vue'
import { useSocketStore } from '@/stores/sockets'

describe('ConnectionStatusStrip', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders the dashboard connection card with only management channels', () => {
    const socketStore = useSocketStore()
    socketStore.snapshots.events.status = 'authenticated'
    socketStore.snapshots.logs.status = 'reconnecting'
    socketStore.snapshots.logs.lastError = 'logs 连接异常'
    socketStore.snapshots.pluginConsole.status = 'auth_failed'

    const wrapper = mount(ConnectionStatusStrip, {
      global: {
        plugins: [Antd],
      },
    })

    expect(wrapper.find('[data-testid="dashboard-connection-card"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('连接状态')
    expect(wrapper.text()).toContain('事件和日志连接')
    expect(wrapper.text()).toContain('事件流')
    expect(wrapper.text()).toContain('已认证')
    expect(wrapper.text()).toContain('重连中')
    expect(wrapper.text()).toContain('日志流')
    expect(wrapper.text()).not.toContain('控制台')
    expect(wrapper.text()).not.toContain('pluginConsole')
    expect(wrapper.text()).not.toContain('logs 连接异常')
    expect(wrapper.text()).toContain('重新连接')
  })
})
