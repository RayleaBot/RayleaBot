import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useConfigStore } from '@/stores/config'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('config store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('loads config snapshot into store state', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({
      config: { schema_version: '2', onebot: { ws_url: '' } },
      redacted_fields: ['onebot.access_token'],
    })))

    const store = useConfigStore()
    await store.fetchConfig()

    expect(store.document).toEqual({ schema_version: '2', onebot: { ws_url: '' } })
    expect(store.redactedFields).toEqual(['onebot.access_token'])
    expect(store.restartRequired).toBeNull()
  })

  it('saves config and updates restart_required', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({
      config: { schema_version: '2', onebot: { ws_url: 'ws://127.0.0.1:2658' } },
      redacted_fields: [],
      restart_required: true,
    })))

    const store = useConfigStore()
    const response = await store.saveConfig({ schema_version: '2', onebot: { ws_url: 'ws://127.0.0.1:2658' } })

    expect(response.restart_required).toBe(true)
    expect(store.document).toEqual({ schema_version: '2', onebot: { ws_url: 'ws://127.0.0.1:2658' } })
    expect(store.restartRequired).toBe(true)
  })

  it('maps save failures into a visible error', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({
      error: {
        code: 'platform.invalid_request',
        message: '配置校验失败',
        request_id: 'req_cfg_1',
      },
    }, 400)))

    const store = useConfigStore()
    await expect(store.saveConfig({ schema_version: '2' })).rejects.toMatchObject({ code: 'platform.invalid_request' })
    expect(store.error).toBe('请求参数不正确，请检查后重试。')
  })
})
