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
})
