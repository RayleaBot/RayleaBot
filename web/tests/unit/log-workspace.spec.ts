import { createPinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import ManagementLogDetailDrawer from '@/components/logs/ManagementLogDetailDrawer.vue'
import ManagementLogFilters from '@/components/logs/ManagementLogFilters.vue'
import ManagementLogWorkspace from '@/components/logs/ManagementLogWorkspace.vue'
import { useLogHistoryStore } from '@/stores/log-history'
import { useLogsStore } from '@/stores/logs'
import { usePluginsStore } from '@/stores/plugins'
import { useUiShellStore } from '@/stores/ui-shell'
import LogsView from '@/views/operations/LogsView.vue'
import LogsHistoryView from '@/views/operations/LogsHistoryView.vue'

afterEach(() => { vi.unstubAllGlobals() })

it('deactivates the live workspace without rewriting the destination history tab', async () => {
  const pinia = createPinia()
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/logs', name: 'logs', component: LogsView },
    { path: '/logs/history', name: 'logs-history', component: LogsHistoryView },
  ] })
  const liveStore = useLogsStore(pinia)
  const historyStore = useLogHistoryStore(pinia)
  liveStore.filters = { levels: ['info'] }
  liveStore.initialized = true
  historyStore.filters = { source: 'plugin' }
  historyStore.initialized = true
  vi.spyOn(liveStore, 'ensureLoaded').mockResolvedValue([])
  vi.spyOn(historyStore, 'applyFilters').mockResolvedValue([])
  vi.spyOn(usePluginsStore(pinia), 'fetchList').mockResolvedValue(undefined)
  vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => { callback(0); return 0 })
  const shell = useUiShellStore(pinia)
  const destination = '/logs/history?source=plugin&log_id=history&start_at=2026-04-01T00:00:00Z&end_at=2026-04-02T00:00:00Z'
  shell.upsertTab({ name: 'logs', title: 'Live', path: '/logs', fullPath: '/logs?level=info&log_id=live', keepAlive: true })
  shell.upsertTab({ name: 'logs-history', title: 'History', path: '/logs/history', fullPath: destination, keepAlive: true })
  await router.push('/logs?level=info&log_id=live')
  const wrapper = mount({ template: '<RouterView v-slot="{ Component }"><KeepAlive><component :is="Component" /></KeepAlive></RouterView>' }, {
    global: { plugins: [pinia, router] },
  })
  try {
    await flushPromises()
    await router.push(destination)
    await flushPromises()
    expect(shell.tabs.find(tab => tab.name === 'logs')?.fullPath).toBe('/logs?level=info')
    expect(shell.tabs.find(tab => tab.name === 'logs-history')?.fullPath).toBe(destination)
    expect(liveStore.active).toBe(false)
    expect(liveStore.filters).toEqual({ levels: ['info'] })
    expect(historyStore.filters).toEqual({ source: 'plugin' })
  } finally {
    wrapper.unmount()
  }
})

describe.each([
  { name: 'logs', path: '/logs', scope: 'current_session' },
  { name: 'logs-history', path: '/logs/history', scope: 'history' },
] as const)('shared $name workspace', ({ name, path, scope }) => {
  it('retains the selected route and visible drawer error, then clears both when filters are applied', async () => {
    const pinia = createPinia()
    const router = createRouter({ history: createMemoryHistory(), routes: [
      { path: '/logs', name: 'logs', component: LogsView },
      { path: '/logs/history', name: 'logs-history', component: LogsHistoryView },
    ] })
    const liveStore = useLogsStore(pinia)
    const historyStore = useLogHistoryStore(pinia)
    const store = scope === 'history' ? historyStore : liveStore
    store.items = [{ log_id: 'selected', timestamp: '2026-04-02T00:00:00Z',
      level: 'warn', source: 'runtime', message: 'selected summary' }]
    store.initialized = true
    vi.spyOn(store, 'applyFilters').mockResolvedValue(store.items)
    vi.spyOn(liveStore, 'ensureLoaded').mockResolvedValue(liveStore.items)
    vi.spyOn(historyStore, 'refreshAnchor').mockResolvedValue(historyStore.items)
    vi.spyOn(usePluginsStore(pinia), 'fetchList').mockResolvedValue(undefined)
    vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => { callback(0); return 0 })
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({
      code: 'platform.internal_error', message: 'detail request failed',
    }), { status: 500, headers: { 'Content-Type': 'application/json' } })))
    await router.push(`${path}?level=warn`)
    const wrapper = mount({ template: '<RouterView />' }, { global: { plugins: [pinia, router] } })
    try {
      await flushPromises()
      expect(wrapper.findComponent(ManagementLogWorkspace).props('scope')).toBe(scope)
      await wrapper.get('.logs-row').trigger('click')
      await flushPromises()
      const drawer = wrapper.findComponent(ManagementLogDetailDrawer)
      expect(drawer.props('open')).toBe(true)
      expect(drawer.props('error')).toBeTruthy()
      expect(drawer.props('loading')).toBe(false)
      expect(router.currentRoute.value.name).toBe(name)
      expect(router.currentRoute.value.query.log_id).toBe('selected')

      wrapper.findComponent(ManagementLogFilters).vm.$emit('apply')
      await flushPromises()
      expect(drawer.props('open')).toBe(false)
      expect(drawer.props('error')).toBeNull()
      expect(router.currentRoute.value.query.log_id).toBeUndefined()
      expect(router.currentRoute.value.query.level).toEqual(['warn'])
      expect(store.items).toHaveLength(1)
    } finally {
      wrapper.unmount()
    }
  })
})
