import { watch } from 'vue'
import { createPinia } from 'pinia'
import { createApp } from 'vue'

import App from '@/App.vue'
import { i18n } from '@/i18n'
import { installAntDesignVue } from '@/plugins/antd'
import { toLauncherAdmissionHint } from '@/lib/auth-feedback'
import { configureApiRuntime } from '@/request/http'
import { createAppRouter } from '@/router'
import { useAppAvailabilityStore } from '@/stores/app-availability'
import { useSessionStore } from '@/stores/session'
import { useSocketStore } from '@/stores/sockets'
import { useUiShellStore } from '@/stores/ui-shell'
import 'ant-design-vue/dist/reset.css'
import '@/styles/tailwind.css'
import '@/styles/main.scss'

const initialLauncherToken = typeof window === 'undefined'
  ? null
  : new URL(window.location.href).searchParams.get('token')?.trim() || null
const websocketOfflineDelayMs = 2000
const websocketOfflineProbeTimeoutMs = 1500

function currentBrowserPath() {
  if (typeof window === 'undefined') {
    return null
  }

  return `${window.location.pathname}${window.location.search}${window.location.hash}`
}

async function consumeLauncherTokenQuery(
  sessionStore: ReturnType<typeof useSessionStore>,
  launcherToken: string | null,
) {
  if (!launcherToken) {
    return
  }

  const currentUrl = new URL(window.location.href)
  currentUrl.searchParams.delete('token')
  window.history.replaceState({}, '', currentUrl.pathname + currentUrl.search + currentUrl.hash)

  if (sessionStore.setupInitialized === true) {
    try {
      await sessionStore.admitLauncherToken(launcherToken)
    } catch {
      sessionStore.setLauncherAdmissionHint(toLauncherAdmissionHint())
      sessionStore.clearSession()
    }
  }
}

async function syncRouteWithSession(
  router: ReturnType<typeof createAppRouter>,
  sessionStore: ReturnType<typeof useSessionStore>,
  socketStore: ReturnType<typeof useSocketStore>,
) {
  if (!sessionStore.isBootstrapped) {
    return
  }

  if (sessionStore.isAuthenticated) {
    socketStore.ensureManagementSockets()
    const current = router.currentRoute.value
    if (current.name === 'login' || current.name === 'setup') {
      await router.push({ name: 'status' })
    }
    return
  }

  socketStore.disconnectAll()

  const current = router.currentRoute.value
  if (sessionStore.requiresSetup && current.name !== 'setup') {
    await router.push({ name: 'setup' })
    return
  }

  if (!sessionStore.requiresSetup && current.meta.requiresAuth) {
    await router.push({ name: 'login' })
  }
}

function installAvailabilityHandlers(
  router: ReturnType<typeof createAppRouter>,
  sessionStore: ReturnType<typeof useSessionStore>,
  socketStore: ReturnType<typeof useSocketStore>,
  availabilityStore: ReturnType<typeof useAppAvailabilityStore>,
) {
  let websocketOfflineTimer: number | null = null

  function clearWebsocketOfflineTimer() {
    if (websocketOfflineTimer !== null) {
      window.clearTimeout(websocketOfflineTimer)
      websocketOfflineTimer = null
    }
  }

  function openOfflinePage(source: 'browser' | 'http' | 'websocket') {
    const current = router.currentRoute.value
    availabilityStore.markOffline(source, current.name === 'offline' ? availabilityStore.returnPath : current.fullPath)

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
  useUiShellStore(pinia)

  configureApiRuntime({
    getToken: () => sessionStore.token,
    onNetworkUnavailable: () => availabilityStore.markOffline('http', currentBrowserPath()),
    onReachable: () => availabilityStore.markOnline(),
    onUnauthorized: (tokenSnapshot) => sessionStore.handleSessionExpired(tokenSnapshot),
  })

  await sessionStore.bootstrap().catch(() => undefined)
  await consumeLauncherTokenQuery(sessionStore, initialLauncherToken)

  const router = createAppRouter()
  installAvailabilityHandlers(router, sessionStore, socketStore, availabilityStore)
  app.use(router)

  await router.isReady()
  if (availabilityStore.isOffline && router.currentRoute.value.name !== 'offline') {
    await router.replace({ name: 'offline' })
  }
  await syncRouteWithSession(router, sessionStore, socketStore)

  watch(
    () => [sessionStore.isBootstrapped, sessionStore.isAuthenticated, sessionStore.requiresSetup] as const,
    () => syncRouteWithSession(router, sessionStore, socketStore),
  )

  app.mount('#app')
}

void bootstrap()
