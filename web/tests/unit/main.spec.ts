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

vi.mock('ant-design-vue/dist/reset.css', () => ({}))
vi.mock('@/styles/tailwind.css', () => ({}))
vi.mock('@/styles/main.scss', () => ({}))

vi.mock('ant-design-vue', () => ({
  default: {},
}))

vi.mock('@/plugins/antd', () => ({
  installAntDesignVue: vi.fn(),
}))

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

vi.mock('@/request/http', () => ({
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
          name: 'login',
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
      admitLauncherToken: vi.fn().mockResolvedValue(undefined),
      clearSession: vi.fn(),
      handleSessionExpired: vi.fn(),
      setLauncherAdmissionHint: vi.fn(),
    })

    socketStoreFactory.mockReturnValue({
      ensureManagementSockets: vi.fn(),
      disconnectAll: vi.fn(),
      snapshots: {
        events: { status: 'authenticated' },
        tasks: { status: 'authenticated' },
        logs: { status: 'authenticated' },
      },
    })

    appAvailabilityStoreFactory.mockReturnValue({
      isOffline: false,
      returnPath: null,
      markOffline: vi.fn(),
      markOnline: vi.fn(),
    })

    useUiShellStore.mockReturnValue({})
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.useRealTimers()
  })

  it('forwards unauthorized token snapshots to the session store handler', async () => {
    await import('@/main')

    expect(configureApiRuntime).toHaveBeenCalled()

    const runtime = configureApiRuntime.mock.calls
      .map((call) => call[0])
      .find((config) => typeof config.onUnauthorized === 'function')
    const sessionStore = sessionStoreFactory.mock.results[0]?.value

    runtime.onUnauthorized('stale-token')

    expect(sessionStore.handleSessionExpired).toHaveBeenCalledWith('stale-token')
  })

  it('preserves the current deep link when startup detects offline state', async () => {
    window.history.replaceState({}, '', '/plugins/settings?panel=limits#rate')

    await import('@/main')

    const startupRuntime = configureApiRuntime.mock.calls[0]?.[0]
    const availabilityStore = appAvailabilityStoreFactory.mock.results[0]?.value

    startupRuntime.onNetworkUnavailable()

    expect(availabilityStore.markOffline).toHaveBeenCalledWith('http', '/plugins/settings?panel=limits#rate')
  })

  it('does not redirect back to login before consuming a launcher token', async () => {
    const router = {
      currentRoute: {
        value: {
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
      admitLauncherToken: vi.fn().mockImplementation(async () => {
        sessionStore.isAuthenticated = true
        sessionStore.token = 'launcher-session-token'
      }),
      clearSession: vi.fn(),
      handleSessionExpired: vi.fn(),
      setLauncherAdmissionHint: vi.fn(),
    }

    createAppRouter.mockReturnValue(router)
    sessionStoreFactory.mockReturnValue(sessionStore)
    watch.mockImplementation(async (source, callback, options) => {
      if (options?.immediate) {
        await callback(source(), undefined)
      }
    })
    window.history.replaceState({}, '', '/?token=launcher_token_fixture_0001')

    await import('@/main')
    await flushBootstrap()

    expect(sessionStore.admitLauncherToken).toHaveBeenCalledWith('launcher_token_fixture_0001')
    expect(router.push).not.toHaveBeenCalledWith({ name: 'login' })
  })

  it('prefers a fresh launcher token over an existing stale session token', async () => {
    const sessionStore = {
      token: 'stale-session-token',
      isAuthenticated: true,
      isBootstrapped: true,
      requiresSetup: false,
      setupInitialized: true,
      bootstrap: vi.fn().mockResolvedValue(undefined),
      admitLauncherToken: vi.fn().mockImplementation(async () => {
        sessionStore.token = 'launcher-session-token'
      }),
      clearSession: vi.fn(),
      handleSessionExpired: vi.fn(),
      setLauncherAdmissionHint: vi.fn(),
    }

    sessionStoreFactory.mockReturnValue(sessionStore)
    window.history.replaceState({}, '', '/?token=launcher_token_fixture_0001')

    await import('@/main')
    await flushBootstrap()

    expect(sessionStore.admitLauncherToken).toHaveBeenCalledWith('launcher_token_fixture_0001')
    expect(sessionStore.token).toBe('launcher-session-token')
    expect(sessionStore.clearSession).not.toHaveBeenCalled()
  })

  it('opens the offline page after core websocket reconnecting persists', async () => {
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
      admitLauncherToken: vi.fn(),
      clearSession: vi.fn(),
      handleSessionExpired: vi.fn(),
      setLauncherAdmissionHint: vi.fn(),
    }
    const socketStore = {
      ensureManagementSockets: vi.fn(),
      disconnectAll: vi.fn(),
      snapshots: {
        events: { status: 'authenticated' },
        tasks: { status: 'authenticated' },
        logs: { status: 'authenticated' },
      },
    }
    const availabilityStore = {
      isOffline: false,
      returnPath: null,
      markOffline: vi.fn(),
      markOnline: vi.fn(),
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

    const websocketWatcher = watchers.find((item) => Array.isArray(item.source()) && (item.source() as unknown[]).length === 5)
    expect(websocketWatcher).toBeDefined()

    socketStore.snapshots.events.status = 'reconnecting'
    websocketWatcher!.callback(websocketWatcher!.source())

    await vi.advanceTimersByTimeAsync(1999)
    expect(router.replace).not.toHaveBeenCalledWith({ name: 'offline' })

    await vi.advanceTimersByTimeAsync(1)
    expect(availabilityStore.markOffline).toHaveBeenCalledWith('websocket', '/commands')
    expect(router.replace).toHaveBeenCalledWith({ name: 'offline' })
  })

  it('opens the offline page when the background health probe fails', async () => {
    vi.useFakeTimers()
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('offline', { status: 503 })))
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
      admitLauncherToken: vi.fn(),
      clearSession: vi.fn(),
      handleSessionExpired: vi.fn(),
      setLauncherAdmissionHint: vi.fn(),
    }
    const socketStore = {
      ensureManagementSockets: vi.fn(),
      disconnectAll: vi.fn(),
      snapshots: {
        events: { status: 'authenticated' },
        tasks: { status: 'authenticated' },
        logs: { status: 'authenticated' },
      },
    }
    const availabilityStore = {
      isOffline: false,
      returnPath: null,
      markOffline: vi.fn(),
      markOnline: vi.fn(),
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

    await vi.advanceTimersByTimeAsync(2499)
    expect(router.replace).not.toHaveBeenCalledWith({ name: 'offline' })

    await vi.advanceTimersByTimeAsync(1)
    expect(availabilityStore.markOffline).toHaveBeenCalledWith('http', '/plugins')
    expect(router.replace).toHaveBeenCalledWith({ name: 'offline' })
  })

  it('keeps the current page when websocket reconnecting occurs but health is reachable', async () => {
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
      admitLauncherToken: vi.fn(),
      clearSession: vi.fn(),
      handleSessionExpired: vi.fn(),
      setLauncherAdmissionHint: vi.fn(),
    }
    const socketStore = {
      ensureManagementSockets: vi.fn(),
      disconnectAll: vi.fn(),
      snapshots: {
        events: { status: 'authenticated' },
        tasks: { status: 'authenticated' },
        logs: { status: 'authenticated' },
      },
    }
    const availabilityStore = {
      isOffline: false,
      returnPath: null,
      markOffline: vi.fn(),
      markOnline: vi.fn(),
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

    const websocketWatcher = watchers.find((item) => Array.isArray(item.source()) && (item.source() as unknown[]).length === 5)
    expect(websocketWatcher).toBeDefined()

    socketStore.snapshots.logs.status = 'reconnecting'
    websocketWatcher!.callback(websocketWatcher!.source())

    await vi.advanceTimersByTimeAsync(2000)

    expect(availabilityStore.markOnline).toHaveBeenCalled()
    expect(availabilityStore.markOffline).not.toHaveBeenCalledWith('websocket', '/plugins')
    expect(router.replace).not.toHaveBeenCalledWith({ name: 'offline' })
  })
})
