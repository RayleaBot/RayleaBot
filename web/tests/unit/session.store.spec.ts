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
    window.localStorage.clear()
    window.sessionStorage.clear()
  })

  it('bootstraps setup status', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ initialized: true })))
    const store = useSessionStore()

    await store.bootstrap()

    expect(store.setupInitialized).toBe(true)
  })

  it('does not expose the raw bootstrap failure message as the status hint', async () => {
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

    expect(store.bootstrapError).toBeTruthy()
    expect(store.bootstrapError).not.toContain('当前用户无权执行该操作')
  })

  it('persists token on login', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ session_token: 'fixture-token' })))
    const store = useSessionStore()

    await store.login({ identifier: 'admin', secret: 'fixture-only-secret' })

    expect(store.token).toBe('fixture-token')
    expect(window.localStorage.getItem('rayleabot.session_token')).toBe('fixture-token')
  })

  it('restores token from local storage', () => {
    window.localStorage.setItem('rayleabot.session_token', 'persisted-token')

    const store = useSessionStore()

    expect(store.token).toBe('persisted-token')
    expect(store.isAuthenticated).toBe(true)
  })

  it('clears persisted token storage on session expiration', () => {
    const store = useSessionStore()

    store.setToken('fresh-token')
    store.handleSessionExpired('fresh-token')

    expect(store.token).toBeNull()
    expect(window.localStorage.getItem('rayleabot.session_token')).toBeNull()
  })

  it('ignores session expiration for an older token', () => {
    const store = useSessionStore()

    store.setToken('fresh-token')
    store.handleSessionExpired('stale-token')

    expect(store.token).toBe('fresh-token')
    expect(window.localStorage.getItem('rayleabot.session_token')).toBe('fresh-token')
  })
})
