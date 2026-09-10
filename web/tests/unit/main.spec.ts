import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const configureApiRuntime = vi.fn()
const createAppRouter = vi.fn()
const createApp = vi.fn()
const createPinia = vi.fn()
const useUiShellStore = vi.fn()
const appAvailabilityStoreFactory = vi.fn()
const watch = vi.fn()
const sessionStoreFactory = vi.fn()
const socketStoreFactory = vi.fn()

vi.mock('@/App.vue', () => ({
  default: {},
}))

vi.mock('@/styles/tailwind.css', () => ({}))
vi.mock('@/styles/main.scss', () => ({}))



vi.mock('pinia', () => ({
  createPinia,
}))

vi.mock('@/stores/ui-shell', () => ({
  useUiShellStore,
}))

vi.mock('@/stores/app-availability', () => ({
  useAppAvailabilityStore: appAvailabilityStoreFactory,
}))

vi.mock('vue', () => ({
  createApp,
  watch,
}))

vi.mock('@/router', () => ({
  createAppRouter,
}))

vi.mock('@/lib/http', () => ({
  configureApiRuntime,
}))

vi.mock('@/stores/session', () => ({
  useSessionStore: sessionStoreFactory,
}))

vi.mock('@/stores/sockets', () => ({
  useSocketStore: socketStoreFactory,
}))

describe('web bootstrap', () => {
  async function flushBootstrap() {
    await Promise.resolve()
    await Promise.resolve()
    await Promise.resolve()
    await Promise.resolve()
  }

  beforeEach(() => {
    vi.resetModules()
    vi.clearAllMocks()

    window.history.replaceState({}, '', '/login')

    createPinia.mockReturnValue({})
    createApp.mockReturnValue({
      use: vi.fn().mockReturnThis(),
      mount: vi.fn(),
    })

    createAppRouter.mockReturnValue({
      currentRoute: {
        value: {
          fullPath: '/login',
          name: 'login',
          query: {},
          meta: {},
        },
      },
      isReady: vi.fn().mockResolvedValue(undefined),
      push: vi.fn(),
    })

    sessionStoreFactory.mockReturnValue({
      token: 'fixture-token',
      isAuthenticated: false,
      isBootstrapped: false,
      requiresSetup: false,
      setupInitialized: true,
      bootstrap: vi.fn().mockResolvedValue(undefined),
      clearSession: vi.fn(),
      handleSessionExpired: vi.fn(),
    })

    socketStoreFactory.mockReturnValue({
      ensureManagementSockets: vi.fn(),
      disconnectAll: vi.fn(),
      reconnectAll: vi.fn(),
      snapshots: {
        events: { status: 'authenticated' },
        logs: { status: 'authenticated' },
      },
    })

    appAvailabilityStoreFactory.mockReturnValue({
      isConnectionInterrupted: false,
      markConnected: vi.fn(),
      markConnectionInterrupted: vi.fn(),
    })

    useUiShellStore.mockReturnValue({
      resetRestoredTabs: vi.fn(),
    })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.useRealTimers()
  })

  it('forwards unauthorized responses to the session store handler', async () => {
    await import('@/main')

    expect(configureApiRuntime).toHaveBeenCalled()

    const runtime = configureApiRuntime.mock.calls
      .map((call) => call[0])
      .find((config) => typeof config.onUnauthorized === 'function')
    const sessionStore = sessionStoreFactory.mock.results[0]?.value

    runtime.onUnauthorized()

    expect(sessionStore.handleSessionExpired).toHaveBeenCalledOnce()
    expect(sessionStore.handleSessionExpired).toHaveBeenCalledWith()
  })

  it('keeps workspace state when startup detects a connection interruption', async () => {
    window.history.replaceState({}, '', '/plugins/settings?panel=limits#rate')

    await import('@/main')

    const startupRuntime = configureApiRuntime.mock.calls[0]?.[0]
    const availabilityStore = appAvailabilityStoreFactory.mock.results[0]?.value

    startupRuntime.onNetworkUnavailable()

    expect(availabilityStore.markConnectionInterrupted).toHaveBeenCalledOnce()
    expect(useUiShellStore).not.toHaveBeenCalled()
  })

  it('keeps authenticated startup deep links on the target page', async () => {
    const router = {
      currentRoute: {
        value: {
          fullPath: '/commands',
          name: 'commands',
          meta: { requiresAuth: true },
        },
      },
      isReady: vi.fn().mockResolvedValue(undefined),
      push: vi.fn(),
      replace: vi.fn(),
    }
    const sessionStore = {
      token: 'fixture-token',
      isAuthenticated: true,
      isBootstrapped: true,
      requiresSetup: false,
      setupInitialized: true,
      bootstrap: vi.fn().mockResolvedValue(undefined),
      clearSession: vi.fn(),
      handleSessionExpired: vi.fn(),
    }

    createAppRouter.mockReturnValue(router)
    sessionStoreFactory.mockReturnValue(sessionStore)
    watch.mockImplementation((source, callback, options) => {
      if (options?.immediate) {
        callback(source(), undefined)
      }
    })
    window.history.replaceState({}, '', '/commands')

    await import('@/main')
    await flushBootstrap()

    expect(useUiShellStore).not.toHaveBeenCalled()
    expect(router.replace).not.toHaveBeenCalledWith({ name: 'status' })
  })

  it('mounts the app before startup status and route readiness complete', async () => {
    const pendingBootstrap = new Promise<void>(() => undefined)
    let resolveRouterReady!: () => void
    const routerReady = new Promise<void>((resolve) => {
      resolveRouterReady = resolve
    })
    const app = {
      use: vi.fn().mockReturnThis(),
      mount: vi.fn(),
    }
    const router = {
      currentRoute: {
        value: {
          fullPath: '/',
          name: undefined,
          meta: {},
        },
      },
      isReady: vi.fn().mockReturnValue(routerReady),
      push: vi.fn(),
      replace: vi.fn(),
    }
    const sessionStore = {
      token: null,
      isAuthenticated: false,
      isBootstrapped: false,
      requiresSetup: false,
      setupInitialized: null,
      bootstrap: vi.fn().mockReturnValue(pendingBootstrap),
      clearSession: vi.fn(),
      handleSessionExpired: vi.fn(),
    }

    createApp.mockReturnValue(app)
    createAppRouter.mockReturnValue(router)
    sessionStoreFactory.mockReturnValue(sessionStore)

    await import('@/main')
    await flushBootstrap()

    expect(router.isReady).toHaveBeenCalled()
    expect(app.mount).toHaveBeenCalledWith('#app')

    resolveRouterReady()
    await flushBootstrap()
  })

  it('keeps authenticated startup exception routes in place', async () => {
    const router = {
      currentRoute: {
        value: {
          fullPath: '/',
          name: 'status',
          meta: { requiresAuth: true },
        },
      },
      isReady: vi.fn().mockResolvedValue(undefined),
      push: vi.fn(),
      replace: vi.fn(),
    }
    const sessionStore = {
      token: 'fixture-token',
      isAuthenticated: true,
      isBootstrapped: true,
      requiresSetup: false,
      setupInitialized: true,
      bootstrap: vi.fn().mockResolvedValue(undefined),
      clearSession: vi.fn(),
      handleSessionExpired: vi.fn(),
    }

    createAppRouter.mockReturnValue(router)
    sessionStoreFactory.mockReturnValue(sessionStore)
    watch.mockImplementation((source, callback, options) => {
      if (options?.immediate) {
        callback(source(), undefined)
      }
    })

    await import('@/main')
    await flushBootstrap()

    expect(router.replace).not.toHaveBeenCalledWith({ name: 'status' })
  })

  it('redirects protected startup routes to login with a return target', async () => {
    const router = {
      currentRoute: {
        value: {
          fullPath: '/plugins?token=launcher_token_fixture_0001',
          name: 'status',
          meta: { requiresAuth: true },
        },
      },
      isReady: vi.fn().mockResolvedValue(undefined),
      push: vi.fn(),
    }

    const sessionStore = {
      token: null,
      isAuthenticated: false,
      isBootstrapped: true,
      requiresSetup: false,
      setupInitialized: true,
      bootstrap: vi.fn().mockResolvedValue(undefined),
      clearSession: vi.fn(),
      handleSessionExpired: vi.fn(),
    }

    createAppRouter.mockReturnValue(router)
    sessionStoreFactory.mockReturnValue(sessionStore)
    watch.mockImplementation(async (source, callback, options) => {
      if (options?.immediate) {
        await callback(source(), undefined)
      }
    })
    window.history.replaceState({}, '', '/plugins?token=launcher_token_fixture_0001')

    await import('@/main')
    await flushBootstrap()

    expect(router.push).toHaveBeenCalledWith({
      name: 'login',
      query: { redirect: '/plugins?token=launcher_token_fixture_0001' },
    })
  })

  it('does not treat one reconnecting websocket as a backend outage', async () => {
    vi.useFakeTimers()
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))
    const watchers: Array<{
      callback: (nextValue: unknown, oldValue?: unknown) => unknown
      source: () => unknown
    }> = []
    const router = {
      currentRoute: {
        value: {
          fullPath: '/commands',
          name: 'commands',
          meta: { requiresAuth: true },
        },
      },
      isReady: vi.fn().mockResolvedValue(undefined),
      push: vi.fn(),
      replace: vi.fn(),
    }
    const sessionStore = {
      token: 'fixture-token',
      isAuthenticated: true,
      isBootstrapped: true,
      requiresSetup: false,
      setupInitialized: true,
      bootstrap: vi.fn().mockResolvedValue(undefined),
      clearSession: vi.fn(),
      handleSessionExpired: vi.fn(),
    }
    const socketStore = {
      ensureManagementSockets: vi.fn(),
      disconnectAll: vi.fn(),
      reconnectAll: vi.fn(),
      snapshots: {
        events: { status: 'authenticated' },
        logs: { status: 'authenticated' },
      },
    }
    const availabilityStore = {
      isConnectionInterrupted: false,
      markConnected: vi.fn(),
      markConnectionInterrupted: vi.fn(),
    }

    createAppRouter.mockReturnValue(router)
    sessionStoreFactory.mockReturnValue(sessionStore)
    socketStoreFactory.mockReturnValue(socketStore)
    appAvailabilityStoreFactory.mockReturnValue(availabilityStore)
    watch.mockImplementation((source, callback, options) => {
      watchers.push({ source, callback })
      if (options?.immediate) {
        callback(source(), undefined)
      }
    })

    await import('@/main')
    await flushBootstrap()

    const websocketWatcher = watchers.find((item) => Array.isArray(item.source()) && (item.source() as unknown[]).length === 3)
    expect(websocketWatcher).toBeDefined()

    socketStore.snapshots.events.status = 'reconnecting'
    websocketWatcher!.callback(websocketWatcher!.source())

    await vi.advanceTimersByTimeAsync(3000)

    expect(fetch).not.toHaveBeenCalled()
    expect(availabilityStore.markConnectionInterrupted).not.toHaveBeenCalled()
    expect(router.replace).not.toHaveBeenCalled()
  })

  it('shows the reconnect notice only after an HTTP failure is confirmed', async () => {
    vi.useFakeTimers()
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))
    const router = {
      currentRoute: {
        value: {
          fullPath: '/plugins',
          name: 'plugins',
          meta: { requiresAuth: true },
        },
      },
      isReady: vi.fn().mockResolvedValue(undefined),
      push: vi.fn(),
      replace: vi.fn(),
    }
    const sessionStore = {
      token: 'fixture-token',
      isAuthenticated: true,
      isBootstrapped: true,
      requiresSetup: false,
      setupInitialized: true,
      bootstrap: vi.fn().mockResolvedValue(undefined),
      clearSession: vi.fn(),
      handleSessionExpired: vi.fn(),
    }
    const socketStore = {
      ensureManagementSockets: vi.fn(),
      disconnectAll: vi.fn(),
      reconnectAll: vi.fn(),
      snapshots: {
        events: { status: 'authenticated' },
        logs: { status: 'authenticated' },
      },
    }
    const availabilityStore = {
      isConnectionInterrupted: false,
      markConnected: vi.fn(),
      markConnectionInterrupted: vi.fn(),
    }

    createAppRouter.mockReturnValue(router)
    sessionStoreFactory.mockReturnValue(sessionStore)
    socketStoreFactory.mockReturnValue(socketStore)
    appAvailabilityStoreFactory.mockReturnValue(availabilityStore)
    watch.mockImplementation((source, callback, options) => {
      if (options?.immediate) {
        callback(source(), undefined)
      }
    })

    await import('@/main')
    await flushBootstrap()

    const runtime = configureApiRuntime.mock.calls.at(-1)?.[0]
    runtime.onNetworkUnavailable()

    await vi.advanceTimersByTimeAsync(799)
    expect(availabilityStore.markConnectionInterrupted).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(1)
    expect(availabilityStore.markConnectionInterrupted).toHaveBeenCalledOnce()
    expect(router.replace).not.toHaveBeenCalled()
  })

  it('keeps the current page when both websockets reconnect but health is reachable', async () => {
    vi.useFakeTimers()
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('ok', { status: 200 })))
    const watchers: Array<{
      callback: (nextValue: unknown, oldValue?: unknown) => unknown
      source: () => unknown
    }> = []
    const router = {
      currentRoute: {
        value: {
          fullPath: '/plugins',
          name: 'plugins',
          meta: { requiresAuth: true },
        },
      },
      isReady: vi.fn().mockResolvedValue(undefined),
      push: vi.fn(),
      replace: vi.fn(),
    }
    const sessionStore = {
      token: 'fixture-token',
      isAuthenticated: true,
      isBootstrapped: true,
      requiresSetup: false,
      setupInitialized: true,
      bootstrap: vi.fn().mockResolvedValue(undefined),
      clearSession: vi.fn(),
      handleSessionExpired: vi.fn(),
    }
    const socketStore = {
      ensureManagementSockets: vi.fn(),
      disconnectAll: vi.fn(),
      reconnectAll: vi.fn(),
      snapshots: {
        events: { status: 'authenticated' },
        logs: { status: 'authenticated' },
      },
    }
    const availabilityStore = {
      isConnectionInterrupted: false,
      markConnected: vi.fn(),
      markConnectionInterrupted: vi.fn(),
    }

    createAppRouter.mockReturnValue(router)
    sessionStoreFactory.mockReturnValue(sessionStore)
    socketStoreFactory.mockReturnValue(socketStore)
    appAvailabilityStoreFactory.mockReturnValue(availabilityStore)
    watch.mockImplementation((source, callback, options) => {
      watchers.push({ source, callback })
      if (options?.immediate) {
        callback(source(), undefined)
      }
    })

    await import('@/main')
    await flushBootstrap()

    const websocketWatcher = watchers.find((item) => Array.isArray(item.source()) && (item.source() as unknown[]).length === 3)
    expect(websocketWatcher).toBeDefined()

    socketStore.snapshots.events.status = 'reconnecting'
    socketStore.snapshots.logs.status = 'reconnecting'
    websocketWatcher!.callback(websocketWatcher!.source())

    await vi.advanceTimersByTimeAsync(2500)

    expect(availabilityStore.markConnected).toHaveBeenCalled()
    expect(availabilityStore.markConnectionInterrupted).not.toHaveBeenCalled()
    expect(router.replace).not.toHaveBeenCalled()
  })

  it('automatically reconnects sockets after the backend becomes reachable', async () => {
    vi.useFakeTimers()
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('ok', { status: 200 })))
    const sessionStore = {
      token: 'fixture-token',
      isAuthenticated: true,
      isBootstrapped: true,
      requiresSetup: false,
      setupInitialized: true,
      bootstrap: vi.fn().mockResolvedValue(undefined),
      clearSession: vi.fn(),
      handleSessionExpired: vi.fn(),
    }
    const socketStore = {
      ensureManagementSockets: vi.fn(),
      disconnectAll: vi.fn(),
      reconnectAll: vi.fn(),
      snapshots: {
        events: { status: 'reconnecting' },
        logs: { status: 'reconnecting' },
      },
    }
    const availabilityStore = {
      isConnectionInterrupted: true,
      markConnected: vi.fn(),
      markConnectionInterrupted: vi.fn(),
    }

    sessionStoreFactory.mockReturnValue(sessionStore)
    socketStoreFactory.mockReturnValue(socketStore)
    appAvailabilityStoreFactory.mockReturnValue(availabilityStore)
    watch.mockImplementation((source, callback, options) => {
      if (options?.immediate) {
        callback(source(), undefined)
      }
    })

    await import('@/main')
    await flushBootstrap()

    await vi.advanceTimersByTimeAsync(2500)

    expect(availabilityStore.markConnected).toHaveBeenCalled()
    expect(socketStore.reconnectAll).toHaveBeenCalledOnce()
  })
})
