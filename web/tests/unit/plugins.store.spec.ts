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
        commands: [
          { name: 'weather' },
        ],
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
    expect(store.items[0].commands).toEqual([{ name: 'weather' }])
  })

  it('preserves existing commands when a runtime event only updates states', () => {
    const store = usePluginsStore()

    store.upsert({
      id: 'weather',
      name: 'Weather',
      role: 'user',
      registration_state: 'installed',
      desired_state: 'enabled',
      runtime_state: 'running',
      commands: [{ name: 'weather' }],
      command_conflicts: [],
    })

    store.upsert({
      id: 'weather',
      registration_state: 'installed',
      desired_state: 'enabled',
      runtime_state: 'starting',
    })

    expect(store.items[0].commands).toEqual([{ name: 'weather' }])
  })

  it('keeps grants sorted, merges outbound console logs, and clears both buffers', async () => {
    vi.stubGlobal('fetch', vi.fn()
      .mockResolvedValueOnce(jsonResponse({
        capability: 'render.image',
        state: 'granted',
        source: 'manual',
      }))
      .mockResolvedValueOnce(jsonResponse({
        items: [
          {
            log_id: 'log-1',
            timestamp: '2026-04-05T00:02:00Z',
            level: 'info',
            source: 'adapter.onebot11',
            protocol: 'onebot11',
            plugin_id: 'weather',
            request_id: 'req-1',
            message: 'plugin weather command echo delivered group message: hello',
          },
        ],
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
        timestamp: `2026-04-05T00:${String(Math.floor(index / 60)).padStart(2, '0')}:${String(index % 60).padStart(2, '0')}Z`,
      })
    }

    await store.fetchOutboundConsoleHistory('weather')
    store.appendOutboundLog({
      log_id: 'log-1',
      timestamp: '2026-04-05T00:02:00Z',
      level: 'info',
      source: 'adapter.onebot11',
      protocol: 'onebot11',
      plugin_id: 'weather',
      request_id: 'req-1',
      message: 'plugin weather command echo delivered group message: hello',
    })
    store.appendOutboundLog({
      log_id: 'log-2',
      timestamp: '2026-04-05T00:02:01Z',
      level: 'warn',
      source: 'adapter.onebot11',
      protocol: 'onebot11',
      plugin_id: 'weather',
      request_id: 'req-2',
      message: 'plugin weather command echo failed to deliver group message: hello',
    })

    const frames = store.getConsole('weather')
    expect(frames).toHaveLength(102)
    expect(frames[0].text).toBe('line-5')
    expect(frames[99].text).toBe('line-104')
    expect(frames[100].stream).toBe('outbound')
    expect(frames[100].text).toContain('plugin weather command echo delivered')
    expect(frames[101].stream).toBe('outbound')
    expect(frames[101].text).toContain('failed to deliver')

    store.clearConsole('weather')
    expect(store.getConsole('weather')).toEqual([])
  })
})
