import { createRouter, createWebHistory, type Router, type RouterHistory, type RouteRecordRaw } from 'vue-router'

import { ApiError } from '@/lib/http'
import { readInternalRedirectTarget } from '@/lib/route-redirect'
import { publicRoutes } from '@/router/routes/core'
import { adminRoutes } from '@/router/routes/modules/admin'
import { useAppAvailabilityStore } from '@/stores/app-availability'
import { useSessionStore } from '@/stores/session'
import { useUiShellStore } from '@/stores/ui-shell'

declare module 'vue-router' {
  interface RouteMeta {
    activePath?: string
    affixTab?: boolean
    affixTabOrder?: number
    entryPath?: string
    exceptionStatus?: '403' | '404' | '500'
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
  installRouteErrorHandler(router)
  return router
}

function isRouteAssetLoadError(error: unknown) {
  if (!(error instanceof Error)) {
    return false
  }

  return /dynamically imported module|loading chunk|unable to preload|importing a module script failed/i.test(error.message)
}

function isConnectionUnavailableError(error: unknown) {
  if (error instanceof TypeError) {
    return true
  }

  if (error instanceof ApiError && (error.status === 0 || error.status === 503)) {
    return true
  }

  return error instanceof Error && /failed to fetch|network|load failed/i.test(error.message)
}

function installRouteErrorHandler(router: Router) {
  router.onError((error) => {
    const uiShellStore = useUiShellStore()
    uiShellStore.setRouteLoading(false)

    if (isRouteAssetLoadError(error)) {
      const availabilityStore = useAppAvailabilityStore()
      availabilityStore.markConnectionInterrupted()
      return
    }

    if (router.currentRoute.value.name !== 'server-error') {
      void router.replace({ name: 'server-error' }).catch(() => undefined)
    }
  })
}

function installRouteGuards(router: Router) {
  let loadingTimer: number | null = null

  router.beforeEach(async (to) => {
    const sessionStore = useSessionStore()
    const availabilityStore = useAppAvailabilityStore()
    const uiShellStore = useUiShellStore()

    if (typeof window !== 'undefined' && loadingTimer) {
      window.clearTimeout(loadingTimer)
      loadingTimer = null
    }

    if (to.fullPath !== router.currentRoute.value.fullPath) {
      uiShellStore.setRouteLoading(true)
    }

    if (!sessionStore.isBootstrapped) {
      try {
        await sessionStore.bootstrap()
      } catch (error) {
        if (availabilityStore.isConnectionInterrupted || isConnectionUnavailableError(error)) {
          availabilityStore.markConnectionInterrupted()
          return true
        }

        if (to.meta.requiresAuth) {
          return { name: 'login' }
        }
      }
    }

    if (sessionStore.requiresSetup) {
      sessionStore.clearSession()
      if (to.name !== 'setup') {
        return {
          name: 'setup',
          query: to.fullPath ? { redirect: to.fullPath } : undefined,
        }
      }
      return true
    }

    if (!sessionStore.requiresSetup && to.name === 'setup') {
      return sessionStore.isAuthenticated ? { path: readInternalRedirectTarget(to.query.redirect) ?? '/' } : {
        name: 'login',
        query: to.query.redirect ? { redirect: to.query.redirect } : undefined,
      }
    }

    if (to.meta.requiresAuth && !sessionStore.isAuthenticated) {
      return {
        name: 'login',
        query: { redirect: to.fullPath },
      }
    }

    if (sessionStore.isAuthenticated && (to.name === 'login' || to.name === 'setup')) {
      return { path: readInternalRedirectTarget(to.query.redirect) ?? '/' }
    }

    return true
  })

  router.afterEach(() => {
    const uiShellStore = useUiShellStore()

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
