import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { useAdaptersStore } from '@/stores/adapters'
import { useConfigStore } from '@/stores/config'
import type { AdapterDescriptor, MessageStatsConnection, MessageStatsResponse } from '@/types/api'
import DashboardConnectionsCard from '@/views/dashboard/DashboardConnectionsCard.vue'

const http = vi.hoisted(() => ({ apiRequest: vi.fn() }))
vi.mock('@/lib/http', () => ({ apiRequest: http.apiRequest }))

function adapter(id: string, name: string, overrides: Partial<AdapterDescriptor> = {}): AdapterDescriptor {
  return { id, protocol: 'onebot11', display_name: id, enabled: true, state: 'connected', summary: '已连接。', identity: { id: '10001', name }, ...overrides }
}

function connection(id: string, received: number, overrides: Partial<MessageStatsConnection> = {}): MessageStatsConnection {
  return { adapter_id: id, protocol: 'onebot11', configured: true, received: [received], sent: [0], totals: { received, sent: 0 }, previous: { received, sent: 0 }, ...overrides }
}

function response(connections: MessageStatsConnection[]): MessageStatsResponse {
  const received = connections.reduce((sum, item) => sum + item.totals.received, 0)
  return {
    granularity: 'hour', timezone: 'Asia/Shanghai', start_at: '2026-09-24T16:00:00Z', end_at: '2026-10-01T16:00:00Z', as_of: '2026-10-01T06:30:00Z',
    tracking_started_at: '2026-04-02T02:00:00Z', buckets: ['2026-10-01T06:00:00Z'], totals: { received, sent: 0 }, previous: { received, sent: 0 },
    connections, incidents: [], incidents_truncated: false,
  }
}

async function mountCard(adapters: AdapterDescriptor[], stats: MessageStatsResponse) {
  http.apiRequest.mockResolvedValue(stats)
  const adaptersStore = useAdaptersStore()
  adaptersStore.adapters = adapters
  adaptersStore.loaded = true
  useConfigStore().effectiveTimezone = 'Asia/Shanghai'
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'status', component: { template: '<div />' } },
      { path: '/protocols', name: 'protocols', component: { template: '<div />' } },
      { path: '/logs/history', name: 'logs-history', component: { template: '<div />' } },
    ],
  })
  await router.push('/')
  const wrapper = mount(DashboardConnectionsCard, { attachTo: document.body, global: { plugins: [router] } })
  await flushPromises()
  return { router, wrapper }
}

describe('DashboardConnectionsCard', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    http.apiRequest.mockReset()
  })
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('lists every connection with room to add one while there are at most four', async () => {
    const { wrapper } = await mountCard(
      [adapter('onebot11', '小雷'), adapter('qq-official', '小雷助手', { protocol: 'qqofficial' })],
      response([connection('onebot11', 30), connection('qq-official', 10, { protocol: 'qqofficial' })]),
    )

    const rows = wrapper.findAll('.connection-row')
    expect(rows.map(row => row.find('.connection-row__title').text())).toEqual(['小雷', '小雷助手'])
    expect(rows[0]!.attributes('href')).toBe('/protocols?adapter=onebot11')
    expect(wrapper.text()).toContain('添加机器人连接')
    expect(wrapper.find('.connections-card__more').exists()).toBe(false)
    expect(wrapper.text()).toContain('近 7 天消息')
    expect(wrapper.text()).toContain('40')
    const query = new URLSearchParams(String(http.apiRequest.mock.calls[0]![0]).split('?')[1])
    expect(Object.fromEntries(query)).toMatchObject({ granularity: 'hour', end_at: expect.stringMatching(/T16:00:00Z$/) })
    wrapper.unmount()
  })

  it('keeps its four slots with more connections and opens all of them in a floating list', async () => {
    const adapters = [
      adapter('a', '甲'), adapter('b', '乙'), adapter('c', '丙'),
      adapter('d', '丁', { state: 'reconnecting', summary: '正在重连' }),
      adapter('e', '戊', { enabled: false, state: 'stopped' }),
    ]
    const { wrapper } = await mountCard(adapters, response([
      connection('a', 50), connection('b', 30), connection('c', 40), connection('d', 2), connection('e', 9),
      connection('gone', 7, { configured: false }),
    ]))

    const titles = wrapper.findAll('.connections-card__rows .connection-row__title').map(item => item.text())
    expect(titles).toEqual(['丁', '甲', '丙'])
    const more = wrapper.get('.connections-card__more')
    expect(more.text()).toContain('其他 3 个连接')
    expect(more.text()).toContain('全部 6 个')
    expect(wrapper.text()).not.toContain('添加机器人连接')

    await more.trigger('click')
    await flushPromises()
    const panel = document.body.querySelector('.connections-all')!
    expect(panel.textContent).toContain('全部 6 个连接')
    expect([...panel.querySelectorAll('h4')].map(heading => heading.textContent)).toEqual([
      expect.stringContaining('使用中 · 4'), expect.stringContaining('已停用 · 1'), expect.stringContaining('已移除 · 1'),
    ])
    // A removed connection only keeps its counts, so it has nowhere to link to.
    const removed = [...panel.querySelectorAll('.connection-row')].find(row => row.textContent?.includes('gone'))!
    expect(removed.tagName).toBe('DIV')
    wrapper.unmount()
  })

  it('reads thirty days by the day', async () => {
    const { wrapper } = await mountCard([adapter('onebot11', '小雷')], response([connection('onebot11', 5)]))
    const month = wrapper.findAll('[role="radio"]').find(item => item.text() === '近 30 天')!
    await month.trigger('click')
    await flushPromises()
    const query = new URLSearchParams(String(http.apiRequest.mock.calls.at(-1)![0]).split('?')[1])
    expect(query.get('granularity')).toBe('day')
    wrapper.unmount()
  })

  it('invites adding a connection when there is none', async () => {
    const { wrapper } = await mountCard([], response([]))
    expect(wrapper.text()).toContain('还没有机器人连接')
    expect(wrapper.find('.message-chart').exists()).toBe(false)
    wrapper.unmount()
  })
})
