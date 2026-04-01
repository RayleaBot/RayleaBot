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

  it('submits render preview requests and returns the accepted task id', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ task_id: 'task_render_preview_0001' }, 202)))
    const store = useSystemStore()

    const response = await store.previewRender({
      template: 'help.menu',
      theme: 'default',
      output: 'png',
      data: {
        title: '帮助菜单',
      },
    })

    expect(response.task_id).toBe('task_render_preview_0001')
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
