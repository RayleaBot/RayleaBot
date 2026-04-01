import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import LogsPage from '@/pages/LogsPage.vue'
import { useLogsStore } from '@/stores/logs'

describe('LogsPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders a compact filter toolbar and virtualized internal viewport', async () => {
    const store = useLogsStore()
    store.items = [
      {
        timestamp: '2026-04-02T00:53:16Z',
        level: 'warn',
        source: 'adapter',
        message: 'adapter reconnect scheduled',
      },
    ]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)

    const wrapper = mount(LogsPage, {
      global: {
        plugins: [ElementPlus],
      },
    })

    await flushPromises()

    expect(wrapper.find('.logs-filter-toolbar').exists()).toBe(true)
    expect(wrapper.find('.logs-filter-grid').exists()).toBe(true)
    expect(wrapper.find('.data-viewport').exists()).toBe(true)
    expect(wrapper.find('.log-summary-row').exists()).toBe(true)
    expect(wrapper.find('.log-summary-top').exists()).toBe(true)
    expect(wrapper.find('.log-summary-bottom').exists()).toBe(true)
    expect(wrapper.find('.summary-text-clamp').exists()).toBe(true)
    expect(wrapper.find('.desktop-table').exists()).toBe(false)
  })
})
