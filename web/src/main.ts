import { watch } from 'vue'
import { createPinia } from 'pinia'
import { createApp } from 'vue'

import App from '@/App.vue'
import { i18n } from '@/i18n'
import { installAntDesignVue } from '@/plugins/antd'
import { configureApiRuntime } from '@/request/http'
import { createAppRouter } from '@/router'
import { useAppAvailabilityStore } from '@/stores/app-availability'
import { useSessionStore } from '@/stores/session'
import { useSocketStore } from '@/stores/sockets'
import { useUiShellStore } from '@/stores/ui-shell'
import 'ant-design-vue/dist/reset.css'
import '@/styles/tailwind.css'
import '@/styles/main.scss'

const websocketOfflineDelayMs = 2000
const websocketOfflineProbeTimeoutMs = 1500
const backendAvailabilityProbeIntervalMs = 2500

function currentBrowserPath() {
  if (typeof window === 'undefined') {
    return null
  }

  return `${window.location.pathname}${window.location.search}${window.location.hash}`
}

function readRouteRedirectTarget(value: unknown) {
  const candidate = Array.isArray(value) ? value[0] : value
  if (typeof candidate !== 'string' || !candidate.trim()) {
    return null
  }

  if (!candidate.startsWith('/') || candidate.startsWith('//') || /\\/.test(candidate)) {
    return null
  }

  return candidate
}

function shouldNormalizeStartupRoute(fullPath: string, routeName: unknown) {
  return (fullPath === '' || fullPath === '/')
    && routeName !== 'status'
    && routeName !== 'login'
    && routeName !== 'setup'
    && routeName !== 'offline'
}

async function syncRouteWithSession(
  router: ReturnType<typeof createAppRouter>,
  sessionStore: ReturnType<typeof useSessionStore>,
  socketStore: ReturnType<typeof useSocketStore>,
) {
  if (!sessionStore.isBootstrapped) {
    return
  }

  const current = router.currentRoute.value
  if (sessionStore.requiresSetup) {
    sessionStore.clearSession()
    socketStore.disconnectAll()
    if (current.name !== 'setup') {
      await router.push({
        name: 'setup',
        query: current.fullPath ? { redirect: current.fullPath } : undefined,
      })
    }
    return
  }

  if (sessionStore.isAuthenticated) {
    socketStore.ensureManagementSockets()
    if (current.name === 'login' || current.name === 'setup') {
      await router.push(readRouteRedirectTarget(current.query.redirect) ?? { name: 'status' })
    }
    return
  }

  socketStore.disconnectAll()

  if (!sessionStore.requiresSetup && current.meta.requiresAuth) {
    await router.push({
      name: 'login',
      query: current.fullPath ? { redirect: current.fullPath } : undefined,
    })
  }
}

function installAvailabilityHandlers(
  router: ReturnType<typeof createAppRouter>,
  sessionStore: ReturnType<typeof useSessionStore>,
  socketStore: ReturnType<typeof useSocketStore>,
  availabilityStore: ReturnType<typeof useAppAvailabilityStore>,
  uiShellStore: ReturnType<typeof useUiShellStore>,
) {
  let websocketOfflineTimer: number | null = null
  let backendAvailabilityTimer: number | null = null
  let backendAvailabilityProbeInFlight = false

  function clearWebsocketOfflineTimer() {
    if (websocketOfflineTimer !== null) {
      window.clearTimeout(websocketOfflineTimer)
      websocketOfflineTimer = null
    }
  }

  function clearBackendAvailabilityTimer() {
    if (backendAvailabilityTimer !== null) {
      window.clearInterval(backendAvailabilityTimer)
      backendAvailabilityTimer = null
    }
  }

  function openOfflinePage(source: 'browser' | 'http' | 'websocket') {
    const current = router.currentRoute.value
    uiShellStore.resetRestoredTabs()
    availabilityStore.markOffline(source, current.name === 'offline' ? availabilityStore.returnPath : current.fullPath)
    clearBackendAvailabilityTimer()

    if (current.name !== 'offline') {
      void router.replace({ name: 'offline' })
    }
  }

  async function canReachBackend() {
    const controller = new AbortController()
    const timeoutId = window.setTimeout(() => controller.abort(), websocketOfflineProbeTimeoutMs)
    try {
      const response = await fetch('/healthz', {
        cache: 'no-store',
        signal: controller.signal,
      })
      return response.ok
    } catch {
      return false
    } finally {
      window.clearTimeout(timeoutId)
    }
  }

  async function probeBackendAvailability() {
    if (
      backendAvailabilityProbeInFlight
      || !sessionStore.isAuthenticated
      || availabilityStore.isOffline
      || router.currentRoute.value.name === 'offline'
    ) {
      return
    }

    backendAvailabilityProbeInFlight = true
    try {
      if (!(await canReachBackend())) {
        openOfflinePage('http')
      }
    } finally {
      backendAvailabilityProbeInFlight = false
    }
  }

  function ensureBackendAvailabilityTimer() {
    if (backendAvailabilityTimer !== null) {
      return
    }

    backendAvailabilityTimer = window.setInterval(() => {
      void probeBackendAvailability()
    }, backendAvailabilityProbeIntervalMs)
  }

  configureApiRuntime({
    onNetworkUnavailable: () => openOfflinePage('http'),
    onReachable: () => availabilityStore.markOnline(),
  })

  if (typeof window !== 'undefined') {
    window.addEventListener('offline', () => openOfflinePage('browser'))
  }

  watch(
    () => [
      sessionStore.isAuthenticated,
      socketStore.snapshots.events.status,
      socketStore.snapshots.tasks.status,
      socketStore.snapshots.logs.status,
      router.currentRoute.value.name,
    ] as const,
    ([isAuthenticated, eventsStatus, tasksStatus, logsStatus, routeName]) => {
      const shouldWatchSockets = isAuthenticated && routeName !== 'offline' && !availabilityStore.isOffline
      const hasReconnectingCoreSocket = [eventsStatus, tasksStatus, logsStatus].some((status) => status === 'reconnecting')

      if (!shouldWatchSockets || !hasReconnectingCoreSocket) {
        clearWebsocketOfflineTimer()
        return
      }

      if (websocketOfflineTimer !== null) {
        return
      }

      websocketOfflineTimer = window.setTimeout(async () => {
        websocketOfflineTimer = null
        const coreStatuses = [
          socketStore.snapshots.events.status,
          socketStore.snapshots.tasks.status,
          socketStore.snapshots.logs.status,
        ]

        if (
          !sessionStore.isAuthenticated
          || router.currentRoute.value.name === 'offline'
          || !coreStatuses.some((status) => status === 'reconnecting')
        ) {
          return
        }

        if (await canReachBackend()) {
          availabilityStore.markOnline()
          return
        }

        openOfflinePage('websocket')
      }, websocketOfflineDelayMs)
    },
    { immediate: true },
  )

  watch(
    () => [sessionStore.isAuthenticated, router.currentRoute.value.name, availabilityStore.isOffline] as const,
    ([isAuthenticated, routeName, isOffline]) => {
      if (isAuthenticated && routeName !== 'offline' && !isOffline) {
        ensureBackendAvailabilityTimer()
        return
      }

      clearBackendAvailabilityTimer()
    },
    { immediate: true },
  )
}

async function bootstrap() {
  const app = createApp(App)
  const pinia = createPinia()

  app.use(pinia)
  installAntDesignVue(app)
  app.use(i18n)

  const sessionStore = useSessionStore(pinia)
  const socketStore = useSocketStore(pinia)
  const availabilityStore = useAppAvailabilityStore(pinia)
  const uiShellStore = useUiShellStore(pinia)

  configureApiRuntime({
    getToken: () => sessionStore.token,
    onNetworkUnavailable: () => {
      uiShellStore.resetRestoredTabs()
      availabilityStore.markOffline('http', currentBrowserPath())
    },
    onReachable: () => availabilityStore.markOnline(),
    onUnauthorized: (tokenSnapshot) => sessionStore.handleSessionExpired(tokenSnapshot),
  })

  await sessionStore.bootstrap().catch(() => undefined)
  uiShellStore.resetRestoredTabs()

  const router = createAppRouter()
  installAvailabilityHandlers(router, sessionStore, socketStore, availabilityStore, uiShellStore)
  app.use(router)

  await router.isReady()
  if (availabilityStore.isOffline && router.currentRoute.value.name !== 'offline') {
    await router.replace({ name: 'offline' })
  }
  await syncRouteWithSession(router, sessionStore, socketStore)
  if (
    sessionStore.isAuthenticated
    && !availabilityStore.isOffline
    && shouldNormalizeStartupRoute(router.currentRoute.value.fullPath, router.currentRoute.value.name)
  ) {
    await router.replace({ name: 'status' })
  }

  watch(
    () => [sessionStore.isBootstrapped, sessionStore.isAuthenticated, sessionStore.requiresSetup] as const,
    () => syncRouteWithSession(router, sessionStore, socketStore),
  )

  app.mount('#app')
}

void bootstrap()
