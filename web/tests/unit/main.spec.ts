import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const configureApiRuntime = vi.fn()
const createAppRouter = vi.fn()
const createApp = vi.fn()
const createPinia = vi.fn()
const useUiShellStore = vi.fn()
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
    })

    useUiShellStore.mockReturnValue({})
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('forwards unauthorized token snapshots to the session store handler', async () => {
    await import('@/main')

    expect(configureApiRuntime).toHaveBeenCalledTimes(1)

    const runtime = configureApiRuntime.mock.calls[0]?.[0]
    const sessionStore = sessionStoreFactory.mock.results[0]?.value

    runtime.onUnauthorized('stale-token')

    expect(sessionStore.handleSessionExpired).toHaveBeenCalledWith('stale-token')
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
})
