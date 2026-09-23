import type { Router } from 'vue-router'

import { readInternalRedirectTarget } from '@/lib/route-redirect'
import type { useSessionStore } from '@/stores/session'
import type { useSocketStore } from '@/stores/sockets'

export function shouldNormalizeStartupRoute(fullPath: string, routeName: unknown) {
  return (fullPath === '' || fullPath === '/')
    && routeName !== 'status'
    && routeName !== 'login'
    && routeName !== 'setup'
}

export async function syncRouteWithSession(
  router: Router,
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
