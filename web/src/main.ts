import { watch } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import { createApp } from 'vue'

import App from '@/App.vue'
import { configureApiRuntime } from '@/lib/http'
import { createAppRouter } from '@/router'
import { useSessionStore } from '@/stores/session'
import { useSocketStore } from '@/stores/sockets'
import '@/styles/main.scss'
import 'element-plus/dist/index.css'

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
      sessionStore.setLauncherAdmissionHint('自动登录未完成，请手动登录。')
      sessionStore.clearSession()
    }
  }
}

async function bootstrap() {
  const app = createApp(App)
  const pinia = createPinia()
  const router = createAppRouter()

  app.use(pinia)
  app.use(router)
  app.use(ElementPlus)

  const sessionStore = useSessionStore(pinia)
  const socketStore = useSocketStore(pinia)

  configureApiRuntime({
    getToken: () => sessionStore.token,
    onUnauthorized: () => sessionStore.handleSessionExpired(),
  })

  await sessionStore.bootstrap().catch(() => undefined)

  watch(
    () => [sessionStore.isBootstrapped, sessionStore.isAuthenticated] as const,
    async ([bootstrapped, authenticated]) => {
      if (!bootstrapped) {
        return
      }

      if (authenticated) {
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
    },
    { immediate: true },
  )

  await router.isReady()
  await consumeLauncherTokenQuery(sessionStore, initialLauncherToken)
  const readyRoute = router.currentRoute.value
  if (sessionStore.isAuthenticated && (readyRoute.name === 'login' || readyRoute.name === 'setup')) {
    await router.push({ name: 'status' })
  } else if (sessionStore.requiresSetup && readyRoute.name !== 'setup') {
    await router.push({ name: 'setup' })
  }
  app.mount('#app')
}

void bootstrap()
