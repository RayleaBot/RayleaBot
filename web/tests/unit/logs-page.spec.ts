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
        message: 'runtime bridge queued for dispatcher private message: 6',
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
})
