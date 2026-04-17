import { watch } from 'vue'
import { createPinia } from 'pinia'
import Antd from 'ant-design-vue'
import { createApp } from 'vue'
import { MotionPlugin } from '@vueuse/motion'

import App from '@/App.vue'
import { i18n } from '@/i18n'
import { toLauncherAdmissionHint } from '@/lib/auth-feedback'
import { configureApiRuntime } from '@/request/http'
import { createAppRouter } from '@/router'
import { useSessionStore } from '@/stores/session'
import { useSocketStore } from '@/stores/sockets'
import { useUiShellStore } from '@/stores/ui-shell'
import 'ant-design-vue/dist/reset.css'
import '@/styles/tailwind.css'
import '@/styles/main.scss'

const initialLauncherToken = typeof window === 'undefined'
  ? null
  : new URL(window.location.href).searchParams.get('token')?.trim() || null

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

  if (sessionStore.setupInitialized === true && !sessionStore.isAuthenticated) {
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

async function bootstrap() {
  const app = createApp(App)
  const pinia = createPinia()

  app.use(pinia)
  app.use(Antd)
  app.use(i18n)
  app.use(MotionPlugin)

  const sessionStore = useSessionStore(pinia)
  const socketStore = useSocketStore(pinia)
  useUiShellStore(pinia)

  configureApiRuntime({
    getToken: () => sessionStore.token,
    onUnauthorized: (tokenSnapshot) => sessionStore.handleSessionExpired(tokenSnapshot),
  })

  await sessionStore.bootstrap().catch(() => undefined)
  await consumeLauncherTokenQuery(sessionStore, initialLauncherToken)

  const router = createAppRouter()
  app.use(router)

  await router.isReady()
  await syncRouteWithSession(router, sessionStore, socketStore)

  watch(
    () => [sessionStore.isBootstrapped, sessionStore.isAuthenticated, sessionStore.requiresSetup] as const,
    () => syncRouteWithSession(router, sessionStore, socketStore),
  )

  app.mount('#app')
}

void bootstrap()
