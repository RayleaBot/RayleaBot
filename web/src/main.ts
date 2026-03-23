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
  app.mount('#app')
}

void bootstrap()
