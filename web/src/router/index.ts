import { createRouter, createWebHistory, type Router, type RouterHistory, type RouteRecordRaw } from 'vue-router'

import { useSessionStore } from '@/stores/session'
import { publicRoutes } from '@/router/routes/core'
import { adminRoutes } from '@/router/routes/modules/admin'

declare module 'vue-router' {
  interface RouteMeta {
    activePath?: string
    affixTab?: boolean
    hideInMenu?: boolean
    hideInTab?: boolean
    icon?: string
    keepAlive?: boolean
    order?: number
    public?: boolean
    requiresAuth?: boolean
    title?: string
    titleKey?: string
  }
}

export const routes: RouteRecordRaw[] = [...publicRoutes, ...adminRoutes]

export function createAppRouter(history: RouterHistory = createWebHistory()) {
  const router = createRouter({ history, routes })
  installRouteGuards(router)
  return router
}

function installRouteGuards(router: Router) {
  router.beforeEach(async (to) => {
    const sessionStore = useSessionStore()

    if (!sessionStore.isBootstrapped) {
      try {
        await sessionStore.bootstrap()
      } catch {
        if (to.meta.requiresAuth) {
          return { name: 'login' }
        }
      }
    }

    if (sessionStore.requiresSetup && to.name !== 'setup') {
      return { name: 'setup' }
    }

    if (!sessionStore.requiresSetup && to.name === 'setup') {
      return sessionStore.isAuthenticated ? { name: 'status' } : { name: 'login' }
    }

    if (to.meta.requiresAuth && !sessionStore.isAuthenticated) {
      return { name: 'login' }
    }

    if (sessionStore.isAuthenticated && (to.name === 'login' || to.name === 'setup')) {
      return { name: 'status' }
    }

    return true
  })
}
