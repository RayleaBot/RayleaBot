import { createRouter, createWebHistory, type Router, type RouterHistory, type RouteRecordRaw } from 'vue-router'

import { useSessionStore } from '@/stores/session'
import { useUiShellStore } from '@/stores/ui-shell'
import { publicRoutes } from '@/router/routes/core'
import { adminRoutes } from '@/router/routes/modules/admin'

declare module 'vue-router' {
  interface RouteMeta {
    activePath?: string
    affixTab?: boolean
    affixTabOrder?: number
    entryPath?: string
    hideInBreadcrumb?: boolean
    hideInMenu?: boolean
    hideInTab?: boolean
    icon?: string
    keepAlive?: boolean
    order?: number
    public?: boolean
    requiresAuth?: boolean
    title?: string
    titleKey?: string
    viewKey?: string
  }
}

export const routes: RouteRecordRaw[] = [...publicRoutes, ...adminRoutes]

export function createAppRouter(history: RouterHistory = createWebHistory()) {
  const router = createRouter({ history, routes })
  installRouteGuards(router)
  return router
}

function installRouteGuards(router: Router) {
  let loadingTimer: number | null = null

  router.beforeEach(async (to) => {
    const sessionStore = useSessionStore()
    const uiShellStore = useUiShellStore()

    if (typeof window !== 'undefined' && loadingTimer) {
      window.clearTimeout(loadingTimer)
      loadingTimer = null
    }

    if (to.fullPath !== router.currentRoute.value.fullPath && uiShellStore.preferences.pageLoading) {
      uiShellStore.setRouteLoading(true)
    }

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

  router.afterEach(() => {
    const uiShellStore = useUiShellStore()

    if (!uiShellStore.preferences.pageLoading) {
      uiShellStore.setRouteLoading(false)
      return
    }

    if (typeof window === 'undefined') {
      uiShellStore.setRouteLoading(false)
      return
    }

    loadingTimer = window.setTimeout(() => {
      uiShellStore.setRouteLoading(false)
      loadingTimer = null
    }, 160)
  })
}
