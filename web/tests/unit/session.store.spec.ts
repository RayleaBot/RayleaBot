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
  it('clears the browser session only after credentials are successfully saved', async () => {
    setActivePinia(createPinia())
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({ transport: 'cookie', csrf_token: 'fixture-only-csrf-token-with-32-characters' }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { code: 'permission.current_secret_invalid', message: 'incorrect current password' } }), { status: 403 }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetchMock)
    const store = useSessionStore()
    await store.login({ identifier: 'admin', secret: 'old-password' })
    await expect(store.updateCredentials({ current_secret: 'incorrect', new_secret: 'new-password' })).rejects.toThrow()
    expect(store.isAuthenticated).toBe(true)
    expect(store.csrfToken).toBeTruthy()
    await store.updateCredentials({ current_secret: 'old-password', new_secret: 'new-password' })
    expect(store.isAuthenticated).toBe(false)
    expect(store.csrfToken).toBeNull()
    const call = fetchMock.mock.calls[2]
    expect(call?.[0]).toBe('/api/account/credentials')
    expect(JSON.parse(call?.[1].body)).toEqual({ current_secret: 'old-password', new_secret: 'new-password' })
  })
  beforeEach(() => {
    setActivePinia(createPinia())
    window.localStorage.clear()
    window.sessionStorage.clear()
    window.history.replaceState({}, '', '/')
  })

  it('bootstraps setup status and restores a cookie session without reading a token', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({ initialized: true }))
      .mockResolvedValueOnce(jsonResponse({ ok: true }))
    vi.stubGlobal('fetch', fetchMock)
    const store = useSessionStore()

    await store.bootstrap()

    expect(store.setupInitialized).toBe(true)
    expect(store.isAuthenticated).toBe(true)
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/system/status', expect.objectContaining({
      credentials: 'same-origin',
    }))
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

  it('keeps cookie authentication and CSRF state only in memory', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      transport: 'cookie',
      csrf_token: 'fixture-csrf-token-with-at-least-32-bytes',
      expires_at: '2026-08-01T00:00:00Z',
    }))
    vi.stubGlobal('fetch', fetchMock)
    const store = useSessionStore()

    await store.login({ identifier: 'admin', secret: 'fixture-only-secret' })

    expect(store.isAuthenticated).toBe(true)
    expect(store.csrfToken).toBe('fixture-csrf-token-with-at-least-32-bytes')
    expect(window.localStorage.getItem('rayleabot.session_token')).toBeNull()
    const request = fetchMock.mock.calls[0]?.[1] as RequestInit
    expect(new Headers(request.headers).get('X-Raylea-Session-Transport')).toBe('cookie')
    expect(request.credentials).toBe('same-origin')
  })

  it('clears in-memory cookie session state on expiration', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({
      transport: 'cookie',
      csrf_token: 'fixture-csrf-token-with-at-least-32-bytes',
    })))
    const store = useSessionStore()

    await store.login({ identifier: 'admin', secret: 'fixture-only-secret' })
    store.handleSessionExpired()

    expect(store.isAuthenticated).toBe(false)
    expect(store.csrfToken).toBeNull()
    expect(window.localStorage.getItem('rayleabot.session_token')).toBeNull()
  })

  it('consumes the one-time setup token from the URL fragment without persisting it', async () => {
    window.history.replaceState({}, '', '/setup#setup_token=fixture-setup-token&source=launcher')
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      transport: 'cookie',
      csrf_token: 'fixture-csrf-token-with-at-least-32-bytes',
    }))
    vi.stubGlobal('fetch', fetchMock)
    const store = useSessionStore()

    expect(window.location.hash).toBe('#source=launcher')
    await store.setupAdmin({ identifier: 'admin', secret: 'fixture-only-secret' })

    const request = fetchMock.mock.calls[0]?.[1] as RequestInit
    expect(new Headers(request.headers).get('X-Raylea-Setup-Token')).toBe('fixture-setup-token')
    expect(window.localStorage.getItem('rayleabot.session_token')).toBeNull()
  })
})
