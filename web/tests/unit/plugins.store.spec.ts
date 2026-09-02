import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { usePluginsStore } from '@/stores/plugins'
import type { PluginDetail } from '@/types/api'

function jsonResponse(body: unknown, status = 200) {
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

    expect(store.sortedItems.map((item) => item.id)).toEqual(['alpha', 'zeta'])
    expect(store.getPluginDisplayName('alpha')).toBe('Alpha')
    expect(store.getPluginDisplayName('unknown')).toBe('unknown')
  })

  it('keeps plugin names for the current session after list state changes', () => {
    const store = usePluginsStore()

    store.upsert({ id: 'weather', name: 'Weather', state: 'running' })
    store.items = []

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
    expect(store.items[0].state).toBe('running')
    expect(store.items[0].commands).toEqual([weatherCommand])
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
        items: [
          {
            id: 'weather',
            name: 'weather',
            role: 'community',
            state: 'disabled',
            commands: [],
          },
        ],
      }))
    vi.stubGlobal('fetch', fetchMock)

    const store = usePluginsStore()
    store.upsert({ id: 'weather', state: 'running' })

    await store.executeAction('weather', 'disable')
    expect(store.items[0].state).toBe('stopping')

    await vi.advanceTimersByTimeAsync(700)

    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(fetchMock.mock.calls[1]?.[0]).toBe('/api/plugins')
    expect(store.items[0].state).toBe('disabled')
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

    expect(store.items[0].commands).toEqual([weatherCommand])
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
