import { createMemoryHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { createAppRouter, routes } from '@/router'
import { useAppAvailabilityStore } from '@/stores/app-availability'
import { useUiShellStore } from '@/stores/ui-shell'

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

function bootstrappedFetch(authenticated: boolean) {
  return vi.fn()
    .mockImplementationOnce(() => Promise.resolve(jsonResponse({ initialized: true })))
    .mockImplementationOnce(() => Promise.resolve(authenticated
      ? jsonResponse({ ok: true })
      : new Response(JSON.stringify({ error: { code: 'permission.denied', message: '需要有效的管理会话' } }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      })))
}

describe('router guards', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    window.localStorage.clear()
    window.sessionStorage.clear()
  })

  it('redirects to setup when setup is required', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ initialized: false })))
    const router = createAppRouter(createMemoryHistory())

    await router.push('/login')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('setup')
  })

  it('redirects protected routes to login when setup is done but session is missing', async () => {
    vi.stubGlobal('fetch', bootstrappedFetch(false))
    const router = createAppRouter(createMemoryHistory())

    await router.push('/plugins')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('login')
  })

  it('registers the split permission pages with the current permission routes', () => {
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

  it('registers fallback routes and redirects unmatched paths to 404', async () => {
    vi.stubGlobal('fetch', bootstrappedFetch(true))
    const router = createAppRouter(createMemoryHistory())

    await router.push('/missing-page')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('not-found')
    expect(router.currentRoute.value.path).toBe('/missing-page')
  })

  it('keeps protected routes open when the Host-only cookie session is valid', async () => {
    vi.stubGlobal('fetch', bootstrappedFetch(true))
    const router = createAppRouter(createMemoryHistory())

    await router.push('/plugins')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('plugins')
  })

  it('opens setup when initialization is required', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ initialized: false })))
    const router = createAppRouter(createMemoryHistory())

    await router.push('/')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('setup')
  })

  it('keeps the requested page in place when session bootstrap is interrupted', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))
    const availabilityStore = useAppAvailabilityStore()
    const router = createAppRouter(createMemoryHistory())

    await router.push('/commands')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('commands')
    expect(availabilityStore.isConnectionInterrupted).toBe(true)
  })

  it('keeps the current page when a route chunk cannot be loaded', async () => {
    vi.useFakeTimers()
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ initialized: true })))
    const availabilityStore = useAppAvailabilityStore()
    const uiShellStore = useUiShellStore()
    uiShellStore.setRouteLoading(true)
    const router = createAppRouter(createMemoryHistory())
    router.addRoute({
      path: '/broken-chunk',
      name: 'broken-chunk',
      component: () => Promise.reject(new Error('Failed to fetch dynamically imported module')),
    })

    try {
      await router.push('/broken-chunk').catch(() => undefined)
      await Promise.resolve()
      await Promise.resolve()
      await vi.advanceTimersByTimeAsync(160)

      expect(availabilityStore.isConnectionInterrupted).toBe(true)
      expect(uiShellStore.routeLoading).toBe(false)
      expect(router.currentRoute.value.name).not.toBe('offline')
    } finally {
      vi.useRealTimers()
    }
  })
})
