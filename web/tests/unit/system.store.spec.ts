import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useSystemStore } from '@/stores/system'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('system store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('marks the system as shutting down after shutdown is accepted', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ accepted: true }, 202)))
    const store = useSystemStore()
    store.system = {
      status: 'running',
      adapters: [{ id: 'onebot11', protocol: 'onebot11', enabled: true, state: 'connected' }],
      active_plugins: 2,
      uptime_seconds: 120,
    }

    await store.requestShutdown()

    expect(store.shutdownRequested).toBe(true)
    expect(store.system?.status).toBe('shutting_down')
  })

  it('accepts readiness payloads that come back with 503 during startup transitions', async () => {
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      const url = typeof input === 'string' ? input : input.toString()

      if (url.endsWith('/healthz')) {
        return jsonResponse({ status: 'ok' })
      }
      if (url.endsWith('/readyz')) {
        return jsonResponse({ status: 'failed', reason: '服务仍在完成启动。' }, 503)
      }
      if (url.endsWith('/api/system/status')) {
        return jsonResponse({
          status: 'running',
          adapters: [{ id: 'onebot11', protocol: 'onebot11', enabled: true, state: 'idle' }],
          active_plugins: 0,
          uptime_seconds: 3,
        })
      }
      if (url.endsWith('/api/system/diagnostics')) {
        return jsonResponse({
          generated_at: '2026-06-25T00:00:00Z',
          issues: [],
        })
      }

      throw new Error(`unexpected url: ${url}`)
    }))

    const store = useSystemStore()
    await store.refresh()

    expect(store.health?.status).toBe('ok')
    expect(store.readiness?.status).toBe('failed')
    expect(store.readiness?.reason).toBe('服务仍在完成启动。')
    expect(store.system?.status).toBe('running')
    expect(store.error).toBeNull()
  })

  it('drops bridge aggregate observability events from the homepage feed', () => {
    const store = useSystemStore()

    store.applyEvent('2026-04-08T10:18:00Z', {
      observability_scope: 'bridge_runtime',
      summary: 'bridge delivered recent adapter events while keeping bridge/runtime observability aggregate-only',
      delivered_count: 3,
      result_count: 3,
      error_count: 0,
    })

    expect(store.recentEvents).toHaveLength(0)
  })

  it('does not overwrite newer status with an older completed request', async () => {
    const responses: Array<(response: Response) => void> = []
    vi.stubGlobal('fetch', vi.fn(() => new Promise<Response>(resolve => { responses.push(resolve) })))
    const store = useSystemStore()
    const first = store.refreshStatus()
    const second = store.refreshStatus()
    const finish = (offset: number, uptime: number) => {
      responses[offset](jsonResponse({ status: 'ready' }))
      responses[offset + 1](jsonResponse({ status: 'running', adapters: [], uptime_seconds: uptime }))
      responses[offset + 2](jsonResponse({ generated_at: String(uptime), issues: [] }))
    }
    finish(3, 2)
    await second
    finish(0, 1)
    await first
    expect(store.system?.uptime_seconds).toBe(2)
  })
})
