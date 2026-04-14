import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { formatDateTime } from '@/lib/format'
import LogsPage from '@/views/operations/LogsView.vue'
import { useLogsStore } from '@/stores/logs'

describe('LogsPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders a compact filter toolbar and structured logs table', async () => {
    const store = useLogsStore()
    store.items = [
      {
        log_id: 'log_warn_0001',
        timestamp: '2026-04-02T00:53:16Z',
        level: 'warn',
        source: 'adapter',
        message: 'adapter reconnect scheduled',
      },
    ]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)

    const wrapper = mount(LogsPage, {
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()

    expect(wrapper.find('.logs-filter-toolbar').exists()).toBe(true)
    expect(wrapper.find('.logs-filter-grid').exists()).toBe(true)
    expect(wrapper.find('.logs-data-table').exists()).toBe(true)
    expect(wrapper.find('.log-cell-time').exists()).toBe(true)
    expect(wrapper.find('.log-cell-source').exists()).toBe(true)
    expect(wrapper.find('.log-message-text').exists()).toBe(true)
    expect(wrapper.find('.desktop-table').exists()).toBe(false)
  })

  it('formats scientific-notation timestamps in the logs table', async () => {
    const store = useLogsStore()
    store.items = [
      {
        log_id: 'log_warn_0002',
        timestamp: '1.775762955e+09',
        level: 'warn',
        source: 'bridge',
        message: '10001: 乔温迪乔斯达(3599026669): 6',
      },
    ]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)

    const wrapper = mount(LogsPage, {
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()

    expect(wrapper.find('.log-time-display').text()).toBe(formatDateTime('1.775762955e+09'))
  })

  it('escapes directional control characters in log messages', async () => {
    const store = useLogsStore()
    store.items = [
      {
        log_id: 'log_warn_unsafe_0001',
        timestamp: '2026-04-14T02:49:45Z',
        level: 'warn',
        source: 'bridge',
        message: '721011692: [760384342]群星怒\u2066，大明云玩家\u202e~喵\u2069(2896109796): 除了战猎这种抓不到加费就完全没法打的角色',
      },
    ]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)

    const wrapper = mount(LogsPage, {
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()

    const message = wrapper.find('.log-message-text').text()
    expect(message).toContain('\\u2066')
    expect(message).toContain('\\u202e')
    expect(message).not.toContain('\u2066')
    expect(message).not.toContain('\u202e')
  })
})
