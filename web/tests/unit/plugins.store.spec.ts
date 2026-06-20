import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { usePluginsStore } from '@/stores/plugins'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
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

    store.upsert({ id: 'zeta', state: 'disabled' })
    store.upsert({ id: 'alpha', state: 'disabled' })

    expect(store.sortedItems.map((item) => item.id)).toEqual(['alpha', 'zeta'])
  })

  it('updates pending action state and plugin snapshot around actions', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({
      plugin: {
        id: 'weather',
        name: 'weather',
        role: 'user',
        state: 'running',
        commands: [
          { name: 'weather', command_source: 'manifest' },
        ],
      },
    })))

    const store = usePluginsStore()
    store.upsert({ id: 'weather', state: 'disabled' })

    const promise = store.executeAction('weather', 'enable')
    expect(store.actionPending.weather).toBe('enable')
    await promise

    expect(store.actionPending.weather).toBeNull()
    expect(store.items[0].state).toBe('running')
    expect(store.items[0].commands).toEqual([{ name: 'weather', command_source: 'manifest' }])
  })

  it('refreshes transient lifecycle state after an accepted action', async () => {
    vi.useFakeTimers()
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({
        plugin: {
          id: 'weather',
          name: 'weather',
          role: 'user',
          state: 'stopping',
          commands: [],
        },
      }))
      .mockResolvedValueOnce(jsonResponse({
        items: [
          {
            id: 'weather',
            name: 'weather',
            role: 'user',
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
      role: 'user',
      state: 'running',
      commands: [{ name: 'weather', command_source: 'manifest' }],
      command_conflicts: [],
    })

    store.upsert({
      id: 'weather',
      state: 'starting',
    })

    expect(store.items[0].commands).toEqual([{ name: 'weather', command_source: 'manifest' }])
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
        role: 'user',
        state: 'running',
      },
    }))
    await secondRequest

    pendingResponses[0]?.(jsonResponse({
      plugin: {
        id: 'weather',
        name: 'Weather',
        role: 'user',
        state: 'disabled',
      },
    }))
    await firstRequest

    expect(store.current?.id).toBe('calendar')
    expect(store.current?.name).toBe('Calendar')
  })
})
