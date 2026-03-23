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
})
