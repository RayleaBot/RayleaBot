import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import VirtualDataViewport from '@/components/VirtualDataViewport.vue'
import { useLogsStore } from '@/stores/logs'
import LogsPage from '@/views/operations/LogsView.vue'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function mockRect(element: Element, width: number, height: number, left = 0, top = 0) {
  Object.defineProperty(element, 'getBoundingClientRect', {
    configurable: true,
    value: () => ({
      x: left,
      y: top,
      width,
      height,
      left,
      top,
      right: left + width,
      bottom: top + height,
      toJSON() {
        return {}
      },
    }),
  })
}

describe('LogsPage', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    setActivePinia(createPinia())
  })

  it('renders the current-session feed and opens the shared detail window', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      log_id: 'log_warn_0001',
      timestamp: '2026-04-02T00:53:16Z',
      level: 'warn',
      source: 'adapter',
      message: 'adapter reconnect scheduled',
      details: {
        retry_in_seconds: 5,
      },
    }))
    vi.stubGlobal('fetch', fetchMock)

    const store = useLogsStore()
    store.items = [
      {
        log_id: 'log_warn_0001',
        timestamp: '2026-04-02T00:53:16Z',
        level: 'warn',
        protocol: 'onebot11',
        source: 'adapter',
        plugin_id: 'weather',
        request_id: 'req_1',
        message: 'adapter reconnect scheduled',
      },
    ]
    vi.spyOn(store, 'ensureLoaded').mockResolvedValue(store.items)

    const wrapper = mount(LogsPage, {
      attachTo: document.body,
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()
    mockRect(wrapper.get('.logs-layout').element, 1600, 960)

    expect(wrapper.text()).toContain('本次服务端启动以来的日志')
    expect(wrapper.text()).toContain('跟随最新')
    expect(wrapper.findComponent(VirtualDataViewport).props('dynamicItemHeight')).toBe(true)
    expect(wrapper.findComponent(VirtualDataViewport).props('bottomThreshold')).toBe(0)
    expect(wrapper.findAll('.logs-row')).toHaveLength(1)

    await wrapper.get('.logs-row').trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith('/api/logs/log_warn_0001', expect.any(Object))
    expect(wrapper.find('.log-detail-window').exists()).toBe(true)
    expect(wrapper.text()).toContain('日志详情')
    expect(wrapper.text()).toContain('详情 JSON')
    expect(wrapper.text()).toContain('weather')
  })

  it('shows pending live rows away from the bottom and jumps back to latest', async () => {
    const store = useLogsStore()
    store.items = [
      {
        log_id: 'log_info_0001',
        timestamp: '2026-04-02T00:53:16Z',
        level: 'info',
        source: 'runtime',
        message: 'runtime ready',
      },
    ]
    store.pendingNewCount = 2
    store.atBottom = false

    vi.spyOn(store, 'ensureLoaded').mockResolvedValue(store.items)
    const acknowledgeSpy = vi.spyOn(store, 'acknowledgePendingNew')
    const bottomSpy = vi.spyOn(store, 'setViewportAtBottom')

    const wrapper = mount(LogsPage, {
      attachTo: document.body,
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()

    expect(wrapper.find('.logs-jump-latest').exists()).toBe(true)
    expect(wrapper.find('.logs-jump-latest').text()).toContain('2')
    await wrapper.get('.logs-jump-latest .ant-btn').trigger('click')
    await flushPromises()

    expect(acknowledgeSpy).toHaveBeenCalledTimes(1)
    expect(bottomSpy).toHaveBeenCalledWith(true)
    expect(store.pendingNewCount).toBe(0)
    expect(store.atBottom).toBe(true)
  })

  it('loads older rows from the top and marks the viewport inactive on unmount', async () => {
    const store = useLogsStore()
    store.items = [
      {
        log_id: 'log_info_0001',
        timestamp: '2026-04-02T00:53:16Z',
        level: 'info',
        source: 'runtime',
        message: 'runtime ready',
      },
    ]
    store.hasOlder = true

    vi.spyOn(store, 'ensureLoaded').mockResolvedValue(store.items)
    const loadOlderSpy = vi.spyOn(store, 'loadOlder').mockResolvedValue(store.items)
    const activeSpy = vi.spyOn(store, 'setViewportActive')

    const wrapper = mount(LogsPage, {
      attachTo: document.body,
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()
    loadOlderSpy.mockClear()
    activeSpy.mockClear()

    wrapper.findComponent(VirtualDataViewport).vm.$emit('reach-top')
    await flushPromises()

    expect(loadOlderSpy).toHaveBeenCalledTimes(1)

    wrapper.unmount()

    expect(activeSpy).toHaveBeenCalledWith(false)
  })
})
