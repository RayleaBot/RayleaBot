import { createMemoryHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { createAppRouter } from '@/router'

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('router guards', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('redirects to setup when setup is required', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ initialized: false })))
    const router = createAppRouter(createMemoryHistory())

    await router.push('/login')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('setup')
  })

  it('redirects protected routes to login when setup is done but session is missing', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ initialized: true })))
    const router = createAppRouter(createMemoryHistory())

    await router.push('/plugins')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('login')
  })
})
