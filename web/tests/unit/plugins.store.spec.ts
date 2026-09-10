import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { usePluginsStore } from '@/stores/plugins'
import type { PluginDetail } from '@/types/api'

function jsonResponse(body: unknown, status = 200) {
  if (body && typeof body === 'object') {
    const value = body as Record<string, unknown>
    if (Array.isArray(value.items)) body = { total: value.items.length, ...value }
    if (Array.isArray(value.user_entries) && Array.isArray(value.group_entries)) body = { total: value.user_entries.length + value.group_entries.length, entry_count: value.user_entries.length + value.group_entries.length, ...value }
  }
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

const weatherCommand = {
  id: 'weather', name: 'weather', effective_names: ['weather'], description: '查询天气', usage: '/weather',
  permission: 'everyone' as const, trigger: { type: 'exact' as const, names: ['weather'] },
}

describe('plugins store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('sorts plugins by id after upsert', () => {
    const store = usePluginsStore()

    store.upsert({ id: 'zeta', name: 'Zeta', state: 'disabled' })
    store.upsert({ id: 'alpha', name: 'Alpha', state: 'disabled' })

    expect(store.knownItems.map((item) => item.id)).toEqual(['alpha', 'zeta'])
    expect(store.getPluginDisplayName('alpha')).toBe('Alpha')
    expect(store.getPluginDisplayName('unknown')).toBe('unknown')
  })

  it('keeps plugin names for the current session after list state changes', () => {
    const store = usePluginsStore()

    store.upsert({ id: 'weather', name: 'Weather', state: 'running' })
    store.items = []

    expect(store.getPluginDisplayName('weather')).toBe('Weather')
  })

  it('updates cached identity and invalidates detail-only fields when the package version changes', () => {
    const store = usePluginsStore()
    const detail = { id: 'weather', name: 'Weather', version: '1', icon: 'old.svg', role: 'community', state: 'running', commands: [], command_groups: [], help: {}, permissions: {}, webhooks: [] } as PluginDetail
    store.items = [detail]
    store.detailsByPluginId = { weather: detail }
    store.upsert({ id: 'weather', name: '新名称', icon: 'new.svg', state: 'running' })
    expect(store.detailsByPluginId.weather?.name).toBe('新名称')
    expect(store.detailsByPluginId.weather?.icon).toBe('new.svg')
    store.upsert({ id: 'weather', version: '2', state: 'running' })
    expect(store.detailsByPluginId.weather).toBeUndefined()
    expect(store.getPluginDisplayName('weather')).toBe('新名称')
  })

  it('re-reads a detail response superseded by a refreshed list', async () => {
    const old = { id: 'weather', name: '旧插件', version: '1', state: 'running', commands: [], command_groups: [], help: {} }
    const fresh = { ...old, name: '新插件', version: '2' }
    let resolveOld: (value: Response) => void = () => {}
    const fetchMock = vi.fn()
      .mockImplementationOnce(() => new Promise<Response>(resolve => { resolveOld = resolve }))
      .mockResolvedValueOnce(jsonResponse({ items: [fresh] }))
      .mockResolvedValueOnce(jsonResponse({ plugin: fresh }))
    vi.stubGlobal('fetch', fetchMock)
    const store = usePluginsStore()
    const pending = store.ensureDetail('weather')
    await store.fetchList()
    resolveOld(jsonResponse({ plugin: old }))
    await pending
    expect(store.detailsByPluginId.weather?.version).toBe('2')
    expect(store.items[0]?.name).toBe('新插件')
  })

  it('removes metadata omitted by a refreshed full detail', async () => {
    const store = usePluginsStore()
    store.upsert({ id: 'weather', name: 'Weather', state: 'running', icon: 'old.svg', description: 'old description' })
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ plugin: { id: 'weather', name: 'Weather', state: 'running', role: 'community', commands: [], command_groups: [], help: {}, permissions: {}, webhooks: [] } })))
    await store.ensureDetail('weather', { refresh: true })
    expect(store.items[0]?.icon).toBeUndefined()
    expect(store.items[0]?.description).toBeUndefined()
  })

  it('refreshes same-version detail metadata after the catalog is refreshed', async () => {
    const store = usePluginsStore()
    const plugin = { id: 'weather', name: 'Weather', version: '1', state: 'running', role: 'community', commands: [], command_groups: [], help: {}, permissions: {}, webhooks: [] }
    vi.stubGlobal('fetch', vi.fn()
      .mockResolvedValueOnce(jsonResponse({ plugin }))
      .mockResolvedValueOnce(jsonResponse({ items: [plugin] }))
      .mockResolvedValueOnce(jsonResponse({ plugin: { ...plugin, repo: 'https://example.test/new-repository' } })))
    await store.ensureDetail('weather')
    await store.fetchList()
    await store.ensureDetail('weather')
    expect(store.detailsByPluginId.weather?.repo).toBe('https://example.test/new-repository')
  })

  it('finishes uninstalling before removing the plugin from the displayed list', async () => {
    const store = usePluginsStore()
    store.upsert({ id: 'weather', name: 'Weather', state: 'running' })
    vi.stubGlobal('fetch', vi.fn()
      .mockResolvedValueOnce(jsonResponse({ task_id: 'uninstall' }, 202))
      .mockResolvedValueOnce(jsonResponse({ task_id: 'uninstall', status: 'succeeded' }))
      .mockResolvedValueOnce(jsonResponse({ items: [] })))
    await store.uninstallPlugin('weather')
    expect(store.items).toEqual([])
    expect(store.actionPending.weather).toBeNull()
    expect(store.getPluginDisplayName('weather')).toBe('Weather')
  })

  it('loads the plugin list once for passive navigation consumers while explicit refresh stays available', async () => {
    const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(jsonResponse({
      items: [{
        id: 'weather',
        name: 'Weather',
        role: 'community',
        state: 'running',
        commands: [],
        help: { groups: [] },
      }],
    })))
    vi.stubGlobal('fetch', fetchMock)
    const store = usePluginsStore()

    await Promise.all([store.ensureList(), store.ensureList()])
    await store.ensureList()

    expect(store.listLoaded).toBe(true)
    expect(fetchMock).toHaveBeenCalledTimes(1)

    await store.fetchList()
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('caches independently requested plugin details without replacing the active detail', async () => {
    const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(jsonResponse({
      plugin: {
        id: 'weather',
        name: 'Weather',
        role: 'community',
        state: 'running',
        commands: [],
        help: { groups: [] },
        management_ui: {
          pages: [{ id: 'settings', label: '天气设置', entry: 'ui/settings.html' }],
        },
      },
    })))
    vi.stubGlobal('fetch', fetchMock)
    const store = usePluginsStore()
    store.current = {
      id: 'echo',
      name: 'Echo',
      role: 'community',
      state: 'running',
      commands: [],
      help: { groups: [] },
    } as PluginDetail

    const firstRequest = store.ensureDetail('weather')
    const secondRequest = store.ensureDetail('weather')
    expect(store.detailLoadingByPluginId.weather).toBe(true)
    await Promise.all([firstRequest, secondRequest])

    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(store.detailsByPluginId.weather?.management_ui?.pages[0]?.id).toBe('settings')
    expect(store.current?.id).toBe('echo')
    expect(store.detailLoadingByPluginId.weather).toBe(false)

    await store.ensureDetail('weather')
    expect(fetchMock).toHaveBeenCalledTimes(1)
    await store.ensureDetail('weather', { refresh: true })
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('updates pending action state and plugin snapshot around actions', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({
      plugin: {
        id: 'weather',
        name: 'weather',
        role: 'community',
        state: 'running',
        commands: [weatherCommand],
        command_groups: [],
        help: {},
      },
    })))

    const store = usePluginsStore()
    store.upsert({ id: 'weather', state: 'disabled' })

    const promise = store.executeAction('weather', 'enable')
    expect(store.actionPending.weather).toBe('enable')
    await promise

    expect(store.actionPending.weather).toBeNull()
    expect(store.knownItems[0].state).toBe('running')
    expect(store.knownItems[0].commands).toEqual([weatherCommand])
  })

  it('refreshes transient lifecycle state after an accepted action', async () => {
    vi.useFakeTimers()
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({
        plugin: {
          id: 'weather',
          name: 'weather',
          role: 'community',
          state: 'stopping',
          commands: [],
        },
      }))
      .mockResolvedValueOnce(jsonResponse({
        plugin: {
            id: 'weather',
            name: 'weather',
            role: 'community',
            state: 'disabled',
            commands: [],
          },
      }))
    vi.stubGlobal('fetch', fetchMock)

    const store = usePluginsStore()
    store.upsert({ id: 'weather', state: 'running' })

    await store.executeAction('weather', 'disable')
    expect(store.knownItems[0].state).toBe('stopping')

    await vi.advanceTimersByTimeAsync(700)

    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(fetchMock.mock.calls[1]?.[0]).toBe('/api/plugins/weather')
    expect(store.knownItems[0].state).toBe('disabled')
  })

  it('preserves existing commands when a runtime event only updates states', () => {
    const store = usePluginsStore()

    store.upsert({
      id: 'weather',
      name: 'Weather',
      role: 'community',
      state: 'running',
      commands: [weatherCommand],
      command_groups: [],
      help: {},
      command_conflicts: [],
    })

    store.upsert({
      id: 'weather',
      state: 'starting',
    })

    expect(store.knownItems[0].commands).toEqual([weatherCommand])
  })

  it('ignores stale plugin detail responses when a newer request is already in flight', async () => {
    const pendingResponses: Array<(response: Response) => void> = []

    vi.stubGlobal('fetch', vi.fn().mockImplementation(() => (
      new Promise<Response>((resolve) => {
        pendingResponses.push(resolve)
      })
    )))

    const store = usePluginsStore()

    const firstRequest = store.fetchDetail('weather')
    const secondRequest = store.fetchDetail('calendar')

    pendingResponses[1]?.(jsonResponse({
      plugin: {
        id: 'calendar',
        name: 'Calendar',
        role: 'community',
        state: 'running',
      },
    }))
    await secondRequest

    pendingResponses[0]?.(jsonResponse({
      plugin: {
        id: 'weather',
        name: 'Weather',
        role: 'community',
        state: 'disabled',
      },
    }))
    await firstRequest

    expect(store.current?.id).toBe('calendar')
    expect(store.current?.name).toBe('Calendar')
  })
})
