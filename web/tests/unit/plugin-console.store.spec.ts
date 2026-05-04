import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { usePluginConsoleStore } from '@/stores/plugin-console'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('plugin console store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('merges process output and outbound logs while keeping bounded buffers', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({
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

    const store = usePluginConsoleStore()

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
