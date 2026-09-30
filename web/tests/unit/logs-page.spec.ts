import { nextTick } from 'vue'
import { createPinia, getActivePinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter, type Router } from 'vue-router'

import VirtualDataViewport from '@/components/VirtualDataViewport.vue'
import { useLogsStore } from '@/stores/logs'
import { usePluginsStore } from '@/stores/plugins'
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

function mountRoutedView(router: Router) {
  return mount({ template: '<RouterView />' }, {
    attachTo: document.body,
    global: {
      plugins: [getActivePinia()!, router],
    },
  })
}

describe('LogsPage', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    setActivePinia(createPinia())
    vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
      callback(0)
      return 0
    })
    const pluginsStore = usePluginsStore()
    pluginsStore.items = [
      {
        id: 'weather',
        name: 'Weather',
        role: 'community',
        state: 'running',
        commands: [],
      },
      {
        id: 'raylea.echo',
        name: 'Echo',
        role: 'community',
        state: 'running',
        commands: [],
      },
    ]
    vi.spyOn(pluginsStore, 'fetchList').mockResolvedValue(undefined)
  })

  function createTestRouter() {
    return createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/logs', name: 'logs', component: LogsPage },
        { path: '/logs/history', name: 'logs-history', component: { template: '<div>history</div>' } },
        { path: '/plugins/:id', name: 'plugin-detail', component: { template: '<div>plugin</div>' } },
        { path: '/protocols', name: 'protocols', component: { template: '<div>protocols</div>' } },
      ],
    })
  }

  it('reads query filters and opens detail with related links', async () => {
    const router = createTestRouter()
    await router.push('/logs?level=warn&level=error&plugin_id=weather&plugin_id=raylea.echo&protocol=onebot11&request_id=req_1')
    await router.isReady()

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
    vi.spyOn(store, 'applyFilters').mockResolvedValue(store.items)
    vi.spyOn(store, 'ensureLoaded').mockResolvedValue(store.items)

    const wrapper = mountRoutedView(router)

    await flushPromises()
    mockRect(wrapper.get('.logs-layout').element, 1600, 960)

    expect(wrapper.text()).toContain('本次服务端启动以来的日志')
    expect(wrapper.text()).toContain('跟随最新')
    expect(wrapper.findComponent(VirtualDataViewport).props('dynamicItemHeight')).toBe(true)
    expect(wrapper.findComponent(VirtualDataViewport).props('itemHeight')).toBe(80)
    expect(wrapper.findComponent(VirtualDataViewport).props('bottomThreshold')).toBe(24)
    expect(wrapper.findAll('.logs-row')).toHaveLength(1)
    expect(store.filters.levels).toEqual(['warn', 'error'])
    expect(store.filters.pluginIds).toEqual(['raylea.echo', 'weather'])
    expect(router.currentRoute.value.query.request_id).toBe('req_1')

    await wrapper.get('.logs-row').trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith('/api/logs/log_warn_0001', expect.any(Object))
    expect(router.currentRoute.value.query.log_id).toBe('log_warn_0001')
    expect(router.currentRoute.value.query.level).toEqual(['warn', 'error'])
    expect(router.currentRoute.value.query.plugin_id).toEqual(['raylea.echo', 'weather'])
    expect(wrapper.find('.log-detail-window').exists()).toBe(true)
    expect(wrapper.text()).toContain('日志详情')
    expect(wrapper.text()).toContain('详情 JSON')
    expect(wrapper.text()).toContain('weather')
    expect(wrapper.text()).toContain('查看插件')
    expect(wrapper.text()).toContain('查看协议')
    expect(wrapper.text()).toContain('相关实时日志')
  })

  it('starts at latest and only shows the jump button after the user leaves the bottom', async () => {
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
    store.initialized = true
    store.pendingNewCount = 2
    store.atBottom = false

    vi.spyOn(store, 'ensureLoaded').mockResolvedValue(store.items)
    const router = createTestRouter()
    await router.push('/logs')
    await router.isReady()

    const wrapper = mountRoutedView(router)

    await flushPromises()

    expect(store.atBottom).toBe(true)
    expect(store.pendingNewCount).toBe(0)
    expect(wrapper.find('.logs-jump-latest').exists()).toBe(false)

    const acknowledgeSpy = vi.spyOn(store, 'acknowledgePendingNew')
    const bottomSpy = vi.spyOn(store, 'setViewportAtBottom')
    store.setViewportAtBottom(false)
    store.pendingNewCount = 2
    await nextTick()

    expect(wrapper.find('.logs-jump-latest').exists()).toBe(true)
    expect(wrapper.find('.logs-jump-latest').text()).toContain('2')
    await wrapper.get('.logs-jump-latest .app-button').trigger('click')
    await flushPromises()

    expect(acknowledgeSpy).toHaveBeenCalledTimes(1)
    expect(bottomSpy).toHaveBeenCalledWith(true)
    expect(store.pendingNewCount).toBe(0)
    expect(store.atBottom).toBe(true)
  })

  it('keeps edited filters as a draft until they are applied', async () => {
    const router = createTestRouter()
    await router.push('/logs')
    await router.isReady()

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
    vi.spyOn(store, 'ensureLoaded').mockResolvedValue(store.items)
    const applyFilters = vi.spyOn(store, 'applyFilters').mockResolvedValue(store.items)

    const wrapper = mountRoutedView(router)
    await flushPromises()

    await wrapper.get('.logs-filter-grid input').setValue('adapter')
    await flushPromises()

    // Streamed logs are still matched against the applied filters while the source is being typed.
    expect(store.filters.source).toBeUndefined()
    expect(store.append({ log_id: 'log_info_0002', timestamp: '2026-04-02T00:53:17Z', level: 'info', source: 'runtime', message: 'still shown' })).toBe(true)
    expect(wrapper.get('.logs-toolbar__pending').text()).toBe('筛选未应用')

    await wrapper.get('.logs-toolbar__apply').trigger('click')
    await flushPromises()

    expect(applyFilters).toHaveBeenCalledTimes(1)
    expect(store.filters.source).toBe('adapter')
    expect(router.currentRoute.value.query.source).toBe('adapter')
    expect(wrapper.get('.logs-toolbar__pending').text()).toBe('')
  })

  it('closes the current log detail and resets live state when leaving the realtime page', async () => {
    const router = createTestRouter()
    await router.push('/logs')
    await router.isReady()

    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      log_id: 'log_info_0001',
      timestamp: '2026-04-02T00:53:16Z',
      level: 'info',
      source: 'runtime',
      message: 'runtime ready',
    }))
    vi.stubGlobal('fetch', fetchMock)

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
    vi.spyOn(store, 'ensureLoaded').mockResolvedValue(store.items)

    const wrapper = mountRoutedView(router)

    await flushPromises()
    mockRect(wrapper.get('.logs-layout').element, 1600, 960)
    await wrapper.get('.logs-row').trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.query.log_id).toBe('log_info_0001')
    expect(wrapper.find('.log-detail-window').exists()).toBe(true)
    // Opening an entry pauses following, so new logs cannot scroll the selected row away.
    expect(store.atBottom).toBe(false)
    expect(wrapper.text()).toContain('已暂停跟随')

    store.pendingNewCount = 1
    await router.push('/protocols')
    await flushPromises()

    expect(router.currentRoute.value.name).toBe('protocols')
    expect(wrapper.find('.log-detail-window').exists()).toBe(false)
    expect(store.active).toBe(false)
    expect(store.atBottom).toBe(true)
    expect(store.pendingNewCount).toBe(0)
  })

  it('does not open a stale detail after realtime log loading finishes on another page', async () => {
    const router = createTestRouter()
    await router.push('/logs?log_id=log_info_0001')
    await router.isReady()

    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      log_id: 'log_info_0001',
      timestamp: '2026-04-02T00:53:16Z',
      level: 'info',
      source: 'runtime',
      message: 'runtime ready',
    }))
    vi.stubGlobal('fetch', fetchMock)

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
    let resolveLoad!: (items: typeof store.items) => void
    vi.spyOn(store, 'ensureLoaded').mockImplementation(() => new Promise((resolve) => {
      resolveLoad = resolve
    }))

    const wrapper = mountRoutedView(router)
    await nextTick()

    await router.push('/protocols')
    resolveLoad(store.items)
    await flushPromises()

    expect(router.currentRoute.value.name).toBe('protocols')
    expect(fetchMock).not.toHaveBeenCalled()
    expect(wrapper.find('.log-detail-window').exists()).toBe(false)
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
    const router = createTestRouter()
    await router.push('/logs')
    await router.isReady()

    const wrapper = mountRoutedView(router)

    await flushPromises()
    loadOlderSpy.mockClear()
    activeSpy.mockClear()

    wrapper.findComponent(VirtualDataViewport).vm.$emit('reach-top')
    await flushPromises()

    expect(loadOlderSpy).toHaveBeenCalledTimes(1)

    wrapper.unmount()

    expect(activeSpy).toHaveBeenCalledWith(false)
  })

  it('does not load a global plugin list for an empty log view', async () => {
    const router = createTestRouter()
    await router.push('/logs')
    await router.isReady()

    const pluginsStore = usePluginsStore()
    pluginsStore.upsert({ id: 'weather', state: 'running' })
    const fetchListSpy = vi.spyOn(pluginsStore, 'fetchList').mockImplementation(async () => {
      pluginsStore.upsert({ id: 'weather', name: '天气插件', state: 'running' })
      pluginsStore.listLoaded = true
    })
    const store = useLogsStore()
    vi.spyOn(store, 'ensureLoaded').mockResolvedValue(store.items)

    mountRoutedView(router)

    await flushPromises()
    expect(fetchListSpy).not.toHaveBeenCalled()
    expect(pluginsStore.getPluginDisplayName('weather')).toBe('Weather')
  })
})
