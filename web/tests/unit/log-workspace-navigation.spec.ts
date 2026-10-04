import { defineComponent, h, nextTick } from 'vue'
import { createPinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import ManagementLogDetailDrawer from '@/components/logs/ManagementLogDetailDrawer.vue'
import { useLogHistoryStore } from '@/stores/log-history'
import { useLogsStore } from '@/stores/logs'
import type { LogSummary } from '@/types/api'
import LogsView from '@/views/operations/LogsView.vue'
import LogsHistoryView from '@/views/operations/LogsHistoryView.vue'

const Viewport = defineComponent({
  name: 'VirtualDataViewport',
  props: { items: { type: Array<LogSummary>, default: () => [] }, followBottom: Boolean },
  emits: ['reach-top', 'at-bottom-change', 'bottom-position-change'],
  setup(props, { emit, expose, slots }) {
    let scrollTop = 580
    const moveAway = () => {
      scrollTop = 0
      emit('at-bottom-change', false)
      emit('bottom-position-change', false)
    }
    const returnToBottom = () => {
      scrollTop = 580
      emit('at-bottom-change', true)
      emit('bottom-position-change', true)
    }
    expose({
      getScrollMetrics: () => ({ clientHeight: 420, scrollHeight: 1000, scrollTop }),
      scrollToBottom: () => {
        scrollTop = 580
        emit('bottom-position-change', true)
        emit('at-bottom-change', true)
      },
    })
    return () => h('div', [
      h('button', { type: 'button', 'data-testid': 'move-away', onClick: moveAway }),
      h('button', { type: 'button', 'data-testid': 'return-bottom', onClick: returnToBottom }),
      ...props.items.map(item => slots.default?.({ item })),
    ])
  },
})

afterEach(() => vi.unstubAllGlobals())

async function workspaceFixture(scope: 'current_session' | 'history', deferred = false) {
  const pinia = createPinia()
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/logs', name: 'logs', component: LogsView },
    { path: '/logs/history', name: 'logs-history', component: LogsHistoryView },
    { path: '/protocols', name: 'protocols', component: { template: '<button data-testid="other-page">Other page</button>' } },
  ] })
  const live = useLogsStore(pinia)
  const history = useLogHistoryStore(pinia)
  const store = scope === 'history' ? history : live
  const selected: LogSummary = { log_id: 'selected', timestamp: '2026-04-02T00:00:00Z', level: 'info', source: 'runtime', message: 'Selected message' }
  store.items = [selected]
  store.initialized = true
  vi.spyOn(live, 'ensureLoaded').mockResolvedValue(live.items)
  vi.spyOn(history, 'refreshAnchor').mockResolvedValue(history.items)
  vi.spyOn(history, 'applyFilters').mockResolvedValue(history.items)
  vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => { callback(0); return 0 })
  const detailResponse = () => new Response(JSON.stringify({ ...selected, details: { message: 'Loaded detail' } }), { headers: { 'Content-Type': 'application/json' } })
  let finishDetail: () => void = () => undefined
  vi.stubGlobal('fetch', deferred
    ? vi.fn().mockImplementation(() => new Promise<Response>(resolve => { finishDetail = () => resolve(detailResponse()) }))
    : vi.fn().mockImplementation(async () => detailResponse()))
  await router.push(scope === 'history' ? '/logs/history' : '/logs')
  await router.isReady()
  const wrapper = mount({ template: '<RouterView v-slot="{ Component }"><KeepAlive><component :is="Component" /></KeepAlive></RouterView>' }, {
    attachTo: document.body,
    global: { plugins: [pinia, router], stubs: { VirtualDataViewport: Viewport } },
  })
  await flushPromises()
  const host = wrapper.get('.logs-layout').element
  Object.defineProperty(host, 'getBoundingClientRect', { configurable: true, value: () => ({ width: 1600, height: 960, left: 0, top: 0 }) })
  return { wrapper, router, live, store, finishDetail: () => finishDetail() }
}

describe.each(['current_session', 'history'] as const)('%s log navigation', (scope) => {
  it('shows the bottom action only when the list is away, including while a detail is selected', async () => {
    const { wrapper, router, live } = await workspaceFixture(scope)
    expect(wrapper.find('.logs-jump-latest').exists()).toBe(false)
    await wrapper.get('.logs-row').trigger('click')
    await flushPromises()
    expect(wrapper.findComponent(ManagementLogDetailDrawer).props('open')).toBe(true)
    expect(wrapper.find('.logs-jump-latest').exists()).toBe(false)
    if (scope === 'current_session') expect(live.atBottom).toBe(false)
    await wrapper.get('[data-testid="move-away"]').trigger('click')
    await nextTick()
    expect(wrapper.find('.logs-jump-latest').exists()).toBe(true)
    await wrapper.get('.logs-jump-latest .app-button').trigger('click')
    await flushPromises()
    expect(wrapper.find('.logs-jump-latest').exists()).toBe(false)
    expect(router.currentRoute.value.query.log_id).toBe('selected')
    expect(wrapper.findComponent(ManagementLogDetailDrawer).props('open')).toBe(true)
    await wrapper.get('[data-testid="move-away"]').trigger('click')
    await nextTick()
    expect(wrapper.find('.logs-jump-latest').exists()).toBe(true)
    await wrapper.get('[data-testid="return-bottom"]').trigger('click')
    await nextTick()
    expect(wrapper.find('.logs-jump-latest').exists()).toBe(false)
  })

  it('closes cached page details on route leave and ignores their late response', async () => {
    const { wrapper, router, finishDetail } = await workspaceFixture(scope, true)
    await wrapper.get('.logs-row').trigger('click')
    await flushPromises()
    const drawer = wrapper.findComponent(ManagementLogDetailDrawer)
    expect(drawer.props('open')).toBe(true)
    expect(drawer.props('loading')).toBe(true)
    expect(wrapper.find('.log-detail-window').exists()).toBe(true)
    await router.push('/protocols')
    await flushPromises()
    expect(drawer.props('open')).toBe(false)
    expect(drawer.props('summary')).toBeNull()
    finishDetail()
    await flushPromises()
    expect(wrapper.find('[data-testid="other-page"]').exists()).toBe(true)
    expect(document.body.querySelector('[data-slot="app-dialog"]')).toBeNull()
    expect(drawer.props('open')).toBe(false)
    expect(drawer.props('detail')).toBeNull()
    expect(drawer.props('loading')).toBe(false)
  })
})
