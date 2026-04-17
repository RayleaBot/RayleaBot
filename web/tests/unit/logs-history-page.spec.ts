import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import VirtualDataViewport from '@/components/VirtualDataViewport.vue'
import { useLogHistoryStore } from '@/stores/log-history'
import LogsHistoryPage from '@/views/operations/LogsHistoryView.vue'

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

describe('LogsHistoryPage', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    setActivePinia(createPinia())
  })

  it('renders the history toolbar and refreshes to a new anchor on mount', async () => {
    const store = useLogHistoryStore()
    store.items = [
      {
        log_id: 'log_history_0001',
        timestamp: '2026-04-02T00:53:16Z',
        level: 'warn',
        source: 'adapter',
        message: 'history row',
      },
    ]
    store.timeRangeInput = {
      startLocal: '2026-04-01T08:00',
      endLocal: '2026-04-02T08:00',
    }
    const refreshSpy = vi.spyOn(store, 'refreshAnchor').mockResolvedValue(store.items)

    const wrapper = mount(LogsHistoryPage, {
      attachTo: document.body,
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()

    expect(refreshSpy).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('历史日志')
    expect(wrapper.text()).toContain('固定时间窗口')
    expect(wrapper.text()).toContain('最近一天')
    expect(wrapper.text()).toContain('最近一周')
    expect(wrapper.text()).toContain('最近一个月')
    expect(wrapper.text()).toContain('最近半年')
    expect(wrapper.findComponent(VirtualDataViewport).props('dynamicItemHeight')).toBe(true)
    expect(wrapper.find('input[type="datetime-local"]').exists()).toBe(true)
  })

  it('opens the shared detail window for a history row', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      log_id: 'log_history_0001',
      timestamp: '2026-04-02T00:53:16Z',
      level: 'warn',
      source: 'adapter',
      message: 'history row',
      details: {
        branch: 'history',
      },
    }))
    vi.stubGlobal('fetch', fetchMock)

    const store = useLogHistoryStore()
    store.items = [
      {
        log_id: 'log_history_0001',
        timestamp: '2026-04-02T00:53:16Z',
        level: 'warn',
        source: 'adapter',
        message: 'history row',
      },
    ]
    vi.spyOn(store, 'refreshAnchor').mockResolvedValue(store.items)

    const wrapper = mount(LogsHistoryPage, {
      attachTo: document.body,
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()
    mockRect(wrapper.get('.logs-layout').element, 1600, 960)
    await wrapper.get('.logs-row').trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith('/api/logs/log_history_0001', expect.any(Object))
    expect(wrapper.find('.log-detail-window').exists()).toBe(true)
    expect(wrapper.text()).toContain('日志详情')
    expect(wrapper.text()).toContain('详情 JSON')
  })

  it('requests older history rows from the top and reuses the recent-day shortcut', async () => {
    const store = useLogHistoryStore()
    store.items = [
      {
        log_id: 'log_history_0001',
        timestamp: '2026-04-02T00:53:16Z',
        level: 'warn',
        source: 'adapter',
        message: 'history row',
      },
    ]
    store.hasOlder = true

    vi.spyOn(store, 'refreshAnchor').mockResolvedValue(store.items)
    const loadOlderSpy = vi.spyOn(store, 'loadOlder').mockResolvedValue(store.items)
    const resetSpy = vi.spyOn(store, 'resetTimeRangeToDefault')

    const wrapper = mount(LogsHistoryPage, {
      attachTo: document.body,
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()
    loadOlderSpy.mockClear()
    wrapper.findComponent(VirtualDataViewport).vm.$emit('reach-top')
    await flushPromises()

    expect(loadOlderSpy).toHaveBeenCalledTimes(1)

    const buttons = wrapper.findAll('button')
    const recentDayButton = buttons.find((candidate) => candidate.text().includes('最近一天'))
    expect(recentDayButton).toBeTruthy()

    await recentDayButton!.trigger('click')
    await flushPromises()

    expect(resetSpy).toHaveBeenCalledTimes(1)

    const setTimeRangeSpy = vi.spyOn(store, 'setTimeRange')
    const weekButton = buttons.find((candidate) => candidate.text().includes('最近一周'))
    expect(weekButton).toBeTruthy()
    await weekButton!.trigger('click')
    await flushPromises()
    expect(setTimeRangeSpy).toHaveBeenCalledWith(7)

    const monthButton = buttons.find((candidate) => candidate.text().includes('最近一个月'))
    expect(monthButton).toBeTruthy()
    await monthButton!.trigger('click')
    await flushPromises()
    expect(setTimeRangeSpy).toHaveBeenCalledWith(30)

    const halfYearButton = buttons.find((candidate) => candidate.text().includes('最近半年'))
    expect(halfYearButton).toBeTruthy()
    await halfYearButton!.trigger('click')
    await flushPromises()
    expect(setTimeRangeSpy).toHaveBeenCalledWith(180)
  })
})
