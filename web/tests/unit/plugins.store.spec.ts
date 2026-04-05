import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

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

  it('sorts plugins by id after upsert', () => {
    const store = usePluginsStore()

    store.upsert({ id: 'zeta', registration_state: 'installed', desired_state: 'disabled', runtime_state: 'stopped' })
    store.upsert({ id: 'alpha', registration_state: 'installed', desired_state: 'disabled', runtime_state: 'stopped' })

    expect(store.sortedItems.map((item) => item.id)).toEqual(['alpha', 'zeta'])
  })

  it('updates pending action state and plugin snapshot around actions', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({
      plugin: {
        id: 'weather',
        name: 'weather',
        role: 'user',
        registration_state: 'installed',
        desired_state: 'enabled',
        runtime_state: 'running',
      },
    })))

    const store = usePluginsStore()
    store.upsert({ id: 'weather', registration_state: 'installed', desired_state: 'disabled', runtime_state: 'stopped' })

    const promise = store.executeAction('weather', 'enable')
    expect(store.actionPending.weather).toBe('enable')
    await promise

    expect(store.actionPending.weather).toBeNull()
    expect(store.items[0].desired_state).toBe('enabled')
    expect(store.items[0].runtime_state).toBe('running')
  })

  it('keeps grants sorted and trims console frames to the latest 100 entries', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({
      capability: 'render.image',
      state: 'granted',
      source: 'manual',
    })))

    const store = usePluginsStore()
    store.grants = {
      weather: [
        { capability: 'scheduler.run', state: 'granted', source: 'manual' },
      ],
    }

    await store.grantCapability('weather', { capability: 'render.image' })
    expect(store.getGrants('weather').map((item) => item.capability)).toEqual(['render.image', 'scheduler.run'])

    for (let index = 0; index < 105; index += 1) {
      store.appendConsole({
        plugin_id: 'weather',
        stream: 'stdout',
        text: `line-${index}`,
        timestamp: `2026-04-05T00:00:${String(index).padStart(2, '0')}Z`,
      })
    }

    const frames = store.getConsole('weather')
    expect(frames).toHaveLength(100)
    expect(frames[0].text).toBe('line-5')
    expect(frames[99].text).toBe('line-104')
  })
})
