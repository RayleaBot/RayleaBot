import { createRouter, createWebHistory, type Router, type RouterHistory, type RouteRecordRaw } from 'vue-router'

import { useSessionStore } from '@/stores/session'
import DashboardPage from '@/pages/DashboardPage.vue'
import ConfigPage from '@/pages/ConfigPage.vue'
import LoginPage from '@/pages/LoginPage.vue'
import LogsPage from '@/pages/LogsPage.vue'
import PluginDetailPage from '@/pages/PluginDetailPage.vue'
import PluginsPage from '@/pages/PluginsPage.vue'
import SetupPage from '@/pages/SetupPage.vue'
import TasksPage from '@/pages/TasksPage.vue'
import AppShell from '@/components/AppShell.vue'

declare module 'vue-router' {
  interface RouteMeta {
    public?: boolean
    requiresAuth?: boolean
    title?: string
  }
}

export const routes: RouteRecordRaw[] = [
  {
    path: '/setup',
    name: 'setup',
    component: SetupPage,
    meta: { public: true, title: '创建管理员账号' },
  },
  {
    path: '/login',
    name: 'login',
    component: LoginPage,
    meta: { public: true, title: '登录' },
  },
  {
    path: '/',
    component: AppShell,
    meta: { requiresAuth: true },
    children: [
      { path: '', name: 'status', component: DashboardPage, meta: { requiresAuth: true, title: '系统状态' } },
      { path: 'plugins', name: 'plugins', component: PluginsPage, meta: { requiresAuth: true, title: '插件' } },
      { path: 'plugins/:id', name: 'plugin-detail', component: PluginDetailPage, meta: { requiresAuth: true, title: '插件详情' } },
      { path: 'tasks', name: 'tasks', component: TasksPage, meta: { requiresAuth: true, title: '任务' } },
      { path: 'logs', name: 'logs', component: LogsPage, meta: { requiresAuth: true, title: '日志' } },
      { path: 'config', name: 'config', component: ConfigPage, meta: { requiresAuth: true, title: '配置' } },
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
