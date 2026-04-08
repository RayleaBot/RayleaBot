import { createRouter, createWebHistory, type Router, type RouterHistory, type RouteRecordRaw } from 'vue-router'

import { useSessionStore } from '@/stores/session'
import DashboardPage from '@/pages/DashboardPage.vue'
import ConfigPage from '@/pages/ConfigPage.vue'
import CommandsPage from '@/pages/CommandsPage.vue'
import LoginPage from '@/pages/LoginPage.vue'
import LogsPage from '@/pages/LogsPage.vue'
import PluginDetailPage from '@/pages/PluginDetailPage.vue'
import PluginsPage from '@/pages/PluginsPage.vue'
import ProtocolLogsPage from '@/pages/ProtocolLogsPage.vue'
import ProtocolsPage from '@/pages/ProtocolsPage.vue'
import SetupPage from '@/pages/SetupPage.vue'
import TasksPage from '@/pages/TasksPage.vue'
import AppShell from '@/components/AppShell.vue'

declare module 'vue-router' {
  interface RouteMeta {
    public?: boolean
    requiresAuth?: boolean
    title?: string
    titleKey?: string
  }
}

export const routes: RouteRecordRaw[] = [
  {
    path: '/setup',
    name: 'setup',
    component: SetupPage,
    meta: { public: true, titleKey: 'routes.setup' },
  },
  {
    path: '/login',
    name: 'login',
    component: LoginPage,
    meta: { public: true, titleKey: 'routes.login' },
  },
  {
    path: '/',
    component: AppShell,
    meta: { requiresAuth: true },
    children: [
      { path: '', name: 'status', component: DashboardPage, meta: { requiresAuth: true, titleKey: 'routes.status' } },
      { path: 'plugins', name: 'plugins', component: PluginsPage, meta: { requiresAuth: true, titleKey: 'routes.plugins' } },
      { path: 'plugins/:id', name: 'plugin-detail', component: PluginDetailPage, meta: { requiresAuth: true, titleKey: 'routes.pluginDetail' } },
      { path: 'commands', name: 'commands', component: CommandsPage, meta: { requiresAuth: true, titleKey: 'routes.commands' } },
      { path: 'tasks', name: 'tasks', component: TasksPage, meta: { requiresAuth: true, titleKey: 'routes.tasks' } },
      { path: 'logs', name: 'logs', component: LogsPage, meta: { requiresAuth: true, titleKey: 'routes.logs' } },
      { path: 'protocols', name: 'protocols', component: ProtocolsPage, meta: { requiresAuth: true, titleKey: 'routes.protocols' } },
      { path: 'protocols/logs', name: 'protocol-logs', component: ProtocolLogsPage, meta: { requiresAuth: true, titleKey: 'routes.protocolLogs' } },
      { path: 'config', name: 'config', component: ConfigPage, meta: { requiresAuth: true, titleKey: 'routes.config' } },
    ],
  },
]

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
