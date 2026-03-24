import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useSessionStore } from '@/stores/session'

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('session store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    window.sessionStorage.clear()
  })

  it('bootstraps setup status', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ initialized: true })))
    const store = useSessionStore()

    await store.bootstrap()

    expect(store.setupInitialized).toBe(true)
  })

  it('maps bootstrap failures to a short chinese status hint', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            error: {
              code: 'permission.denied',
              message: '当前用户无权执行该操作',
            },
          }),
          {
            status: 403,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      ),
    )
    const store = useSessionStore()

    await expect(store.bootstrap()).rejects.toThrow()

    expect(store.bootstrapError).toBe('暂时无法确认管理界面状态，请稍后重试。')
  })

  it('persists token on login', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ session_token: 'fixture-token' })))
    const store = useSessionStore()

    await store.login({ identifier: 'admin', secret: 'fixture-only-secret' })

    expect(store.token).toBe('fixture-token')
    expect(window.sessionStorage.getItem('rayleabot.session_token')).toBe('fixture-token')
  })

  it('persists token on launcher admission', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ session_token: 'launcher-session-token' })))
    const store = useSessionStore()

    await store.admitLauncherToken('launcher_token_fixture_0001')

    expect(store.token).toBe('launcher-session-token')
    expect(window.sessionStorage.getItem('rayleabot.session_token')).toBe('launcher-session-token')
  })

  it('ignores session expiration for an older token', () => {
    const store = useSessionStore()

    store.setToken('fresh-token')
    store.handleSessionExpired('stale-token')

    expect(store.token).toBe('fresh-token')
    expect(window.sessionStorage.getItem('rayleabot.session_token')).toBe('fresh-token')
  })
})
