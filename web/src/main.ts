import { watch } from 'vue'
import { createPinia } from 'pinia'
import { createApp } from 'vue'

import App from '@/App.vue'
import { i18n } from '@/i18n'
import { readInternalRedirectTarget } from '@/lib/route-redirect'
import { configureApiRuntime } from '@/lib/http'
import { createAppRouter } from '@/router'
import { useAppAvailabilityStore } from '@/stores/app-availability'
import { useSessionStore } from '@/stores/session'
import { useSocketStore } from '@/stores/sockets'
import '../../design/typography.generated.css'
import '@/styles/tailwind.css'
import '@/styles/main.scss'

const websocketFailureConfirmationDelayMs = 2500
const requestFailureConfirmationDelayMs = 800
const backendProbeTimeoutMs = 2500
const backendRecoveryProbeIntervalMs = 2500

function shouldNormalizeStartupRoute(fullPath: string, routeName: unknown) {
  return (fullPath === '' || fullPath === '/')
    && routeName !== 'status'
    && routeName !== 'login'
    && routeName !== 'setup'
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
      await router.push(readInternalRedirectTarget(current.query.redirect) ?? { name: 'status' })
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
  sessionStore: ReturnType<typeof useSessionStore>,
  socketStore: ReturnType<typeof useSocketStore>,
  availabilityStore: ReturnType<typeof useAppAvailabilityStore>,
) {
  let requestFailureTimer: number | null = null
  let websocketFailureTimer: number | null = null
  let backendRecoveryTimer: number | null = null
  let backendProbeInFlight = false
  let disposed = false
  const probes = new Set<AbortController>()

  function clearRequestFailureTimer() {
    if (requestFailureTimer !== null) {
      window.clearTimeout(requestFailureTimer)
      requestFailureTimer = null
    }
  }

  function clearWebsocketFailureTimer() {
    if (websocketFailureTimer !== null) {
      window.clearTimeout(websocketFailureTimer)
      websocketFailureTimer = null
    }
  }

  function clearFailureTimers() {
    clearRequestFailureTimer()
    clearWebsocketFailureTimer()
  }

  function clearBackendRecoveryTimer() {
    if (backendRecoveryTimer !== null) {
      window.clearInterval(backendRecoveryTimer)
      backendRecoveryTimer = null
    }
  }

  async function canReachBackend() {
    const controller = new AbortController()
    probes.add(controller)
    const timeoutId = window.setTimeout(() => controller.abort(), backendProbeTimeoutMs)
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
      probes.delete(controller)
    }
  }

  function markConnected() {
    if (disposed) return
    clearFailureTimers()
    clearBackendRecoveryTimer()
    availabilityStore.markConnected()
  }

  async function probeBackendRecovery() {
    if (disposed || backendProbeInFlight || !availabilityStore.isConnectionInterrupted) {
      return
    }

    backendProbeInFlight = true
    try {
      if (!(await canReachBackend())) {
        return
      }

      if (disposed) return
      markConnected()
      if (!sessionStore.isBootstrapped) {
        await sessionStore.bootstrap(true).catch(() => undefined)
      }
      if (sessionStore.isAuthenticated) {
        socketStore.reconnectAll()
      }
    } finally {
      backendProbeInFlight = false
    }
  }

  function ensureBackendRecoveryTimer() {
    if (backendRecoveryTimer !== null) {
      return
    }

    backendRecoveryTimer = window.setInterval(() => {
      void probeBackendRecovery()
    }, backendRecoveryProbeIntervalMs)
  }

  function markConnectionInterrupted() {
    if (disposed) return
    clearFailureTimers()
    availabilityStore.markConnectionInterrupted()
    ensureBackendRecoveryTimer()
  }

  function scheduleConnectionFailureConfirmation(
    source: 'http' | 'websocket',
    delayMs: number,
  ) {
    if (disposed) return
    if (availabilityStore.isConnectionInterrupted) {
      ensureBackendRecoveryTimer()
      return
    }

    const pendingTimer = source === 'http' ? requestFailureTimer : websocketFailureTimer
    if (pendingTimer !== null) {
      return
    }

    const timer = window.setTimeout(async () => {
      if (source === 'http') {
        requestFailureTimer = null
      } else {
        websocketFailureTimer = null
      }

      if (await canReachBackend()) {
        markConnected()
        return
      }

      markConnectionInterrupted()
    }, delayMs)

    if (source === 'http') {
      requestFailureTimer = timer
    } else {
      websocketFailureTimer = timer
    }
  }



  const onOnline = () => { void probeBackendRecovery() }
  if (typeof window !== 'undefined') {
    window.addEventListener('offline', markConnectionInterrupted)
    window.addEventListener('online', onOnline)
  }

  const stopSocketsWatch = watch(
    () => [
      sessionStore.isAuthenticated,
      socketStore.snapshots.events.status,
      socketStore.snapshots.logs.status,
    ] as const,
    ([isAuthenticated, eventsStatus, logsStatus]) => {
      const coreSocketsUnavailable = [eventsStatus, logsStatus].every(
        (status) => status === 'disconnected' || status === 'reconnecting',
      )

      if (!isAuthenticated || !coreSocketsUnavailable) {
        clearWebsocketFailureTimer()
        return
      }

      scheduleConnectionFailureConfirmation('websocket', websocketFailureConfirmationDelayMs)
    },
    { immediate: true },
  )

  const stopAvailabilityWatch = watch(
    () => availabilityStore.isConnectionInterrupted,
    (isConnectionInterrupted) => {
      if (isConnectionInterrupted) {
        ensureBackendRecoveryTimer()
        return
      }

      clearBackendRecoveryTimer()
    },
    { immediate: true },
  )
  return {
    callbacks: {
      onNetworkUnavailable: () => scheduleConnectionFailureConfirmation('http', requestFailureConfirmationDelayMs),
      onReachable: markConnected,
    },
    dispose() {
      disposed = true
      clearFailureTimers()
      clearBackendRecoveryTimer()
      for (const controller of probes) controller.abort()
      probes.clear()
      stopSocketsWatch()
      stopAvailabilityWatch()
      window.removeEventListener('offline', markConnectionInterrupted)
      window.removeEventListener('online', onOnline)
    },
  }
}

async function bootstrap() {
  const app = createApp(App)
  const pinia = createPinia()

  app.use(pinia)
  app.use(i18n)

  const sessionStore = useSessionStore(pinia)
  const socketStore = useSocketStore(pinia)
  const availabilityStore = useAppAvailabilityStore(pinia)

  const availabilityHandlers = installAvailabilityHandlers(sessionStore, socketStore, availabilityStore)
  app.onUnmount(availabilityHandlers.dispose)
  import.meta.hot?.dispose(availabilityHandlers.dispose)
  configureApiRuntime({
    ...availabilityHandlers.callbacks,
    getCSRFToken: () => sessionStore.csrfToken,
    onCSRFToken: (token) => {
      sessionStore.csrfToken = token
    },
    onUnauthorized: () => sessionStore.handleSessionExpired(),
  })

  const router = createAppRouter()
  app.use(router)
  app.mount('#app')

  await router.isReady()
  await syncRouteWithSession(router, sessionStore, socketStore)
  if (
    sessionStore.isAuthenticated
    && shouldNormalizeStartupRoute(router.currentRoute.value.fullPath, router.currentRoute.value.name)
  ) {
    await router.replace({ name: 'status' })
  }

  const stopSessionWatch = watch(
    () => [sessionStore.isBootstrapped, sessionStore.isAuthenticated, sessionStore.requiresSetup] as const,
    () => syncRouteWithSession(router, sessionStore, socketStore),
  )
  app.onUnmount(stopSessionWatch)
  import.meta.hot?.dispose(stopSessionWatch)
}

void bootstrap()
