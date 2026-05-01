import { createMemoryHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { createAppRouter, routes } from '@/router'

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

  it('registers the split permission pages without the legacy governance route', () => {
    const routeNames = new Set<string>()
    const routePaths = new Set<string>()

    function collect(routeList: typeof routes) {
      for (const route of routeList) {
        if (typeof route.name === 'string') {
          routeNames.add(route.name)
        }
        routePaths.add(route.path)
        if (route.children) {
          collect(route.children)
        }
      }
    }

    collect(routes)

    expect(routeNames.has('permission-policy')).toBe(true)
    expect(routeNames.has('access-lists')).toBe(true)
    expect(routeNames.has('governance')).toBe(false)
    expect(routePaths.has('/permission-policy')).toBe(true)
    expect(routePaths.has('/access-lists')).toBe(true)
    expect(routePaths.has('/governance')).toBe(false)
  })
})
