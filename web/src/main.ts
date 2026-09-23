import { createApp, watch } from 'vue'
import { createPinia } from 'pinia'

import App from '@/App.vue'
import { i18n } from '@/i18n'
import { installBackendAvailabilityMonitor } from '@/lib/backend-availability'
import { configureApiRuntime } from '@/lib/http'
import { createAppRouter } from '@/router'
import { shouldNormalizeStartupRoute, syncRouteWithSession } from '@/router/session-sync'
import { useAppAvailabilityStore } from '@/stores/app-availability'
import { useSessionStore } from '@/stores/session'
import { useSocketStore } from '@/stores/sockets'
import '../../design/typography.generated.css'
import '@/styles/tailwind.css'
import '@/styles/main.scss'

async function bootstrap() {
  const app = createApp(App)
  const pinia = createPinia()

  app.use(pinia)
  app.use(i18n)

  const sessionStore = useSessionStore(pinia)
  const socketStore = useSocketStore(pinia)
  const availabilityStore = useAppAvailabilityStore(pinia)

  const availabilityHandlers = installBackendAvailabilityMonitor(sessionStore, socketStore, availabilityStore)
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
