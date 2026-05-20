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
    const config = {
      schema_version: '2',
      onebot: {
        reverse_ws: { enabled: false, url: '', access_token: '' },
        forward_ws: { enabled: false, url: '', access_token: '' },
        http_api: { enabled: false, url: '', access_token: '' },
        webhook: { enabled: false, url: '', access_token: '' },
      },
    }
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({
      config,
      redacted_fields: [],
    })))

    const store = useConfigStore()
    await store.fetchConfig()

    expect(store.document).toEqual(config)
    expect(store.applyEffects).toBeNull()
    expect(store.redactedFields).toEqual([])
    expect(store.restartRequired).toBeNull()
  })

  it('saves config and updates restart_required', async () => {
    const config = {
      schema_version: '2',
      onebot: {
        reverse_ws: { enabled: false, url: '', access_token: '' },
        forward_ws: { enabled: true, url: 'ws://127.0.0.1:2658', access_token: 'forward-secret' },
        http_api: { enabled: false, url: '', access_token: '' },
        webhook: { enabled: false, url: '', access_token: '' },
      },
    }
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({
      config,
      redacted_fields: [],
      restart_required: true,
      apply_effects: {
        applied_now: ['log.level'],
        reloaded_now: [],
        restart_required_fields: ['server.port'],
      },
    })))

    const store = useConfigStore()
    const response = await store.saveConfig(config)

    expect(response.restart_required).toBe(true)
    expect(response.apply_effects.restart_required_fields).toEqual(['server.port'])
    expect(store.applyEffects).toEqual({
      applied_now: ['log.level'],
      reloaded_now: [],
      restart_required_fields: ['server.port'],
    })
    expect(store.document).toEqual(config)
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
