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
      adapter_state: 'ready',
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
          adapter_state: 'idle',
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

  it('formats protocol and plugin events into readable chinese summaries', () => {
    const store = useSystemStore()

    store.applyEvent('2026-04-08T10:16:00Z', {
      connection_status: 'auth_failed',
      summary: 'OneBot authentication failed',
    })
    store.applyEvent('2026-04-08T10:17:00Z', {
      plugin_id: 'weather',
        state: 'running',
    })

    expect(store.recentEvents).toHaveLength(2)
    expect(store.recentEvents[0].summary).toBe('插件 weather 运行中')
    expect(store.recentEvents[1].summary).toBe('协议鉴权失败，请检查访问令牌')
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
})
