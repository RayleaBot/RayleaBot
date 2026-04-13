import type { RouteRecordRaw } from 'vue-router'

import BasicLayout from '@/layouts/BasicLayout.vue'
import RouteView from '@/layouts/RouteView.vue'

export const adminRoutes: RouteRecordRaw[] = [
  {
    path: '/',
    component: BasicLayout,
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        name: 'status',
        component: () => import('@/views/dashboard/DashboardView.vue'),
        meta: {
          affixTab: true,
          icon: 'dashboard',
          order: 1,
          requiresAuth: true,
          titleKey: 'routes.status',
        },
      },
      {
        path: 'plugins',
        name: 'plugins',
        component: () => import('@/views/plugins/PluginsView.vue'),
        meta: {
          icon: 'appstore',
          keepAlive: true,
          order: 2,
          requiresAuth: true,
          titleKey: 'routes.plugins',
        },
      },
      {
        path: 'plugins/:id',
        name: 'plugin-detail',
        component: () => import('@/views/plugins/PluginDetailView.vue'),
        meta: {
          activePath: '/plugins',
          hideInMenu: true,
          requiresAuth: true,
          titleKey: 'routes.pluginDetail',
        },
      },
      {
        path: '',
        component: RouteView,
        redirect: { name: 'commands' },
        meta: {
          hideInTab: true,
          icon: 'toolbox',
          order: 3,
          requiresAuth: true,
          titleKey: 'routes.operations',
        },
        children: [
          {
            path: '/commands',
            name: 'commands',
            component: () => import('@/views/operations/CommandsView.vue'),
            meta: {
              icon: 'commands',
              keepAlive: true,
              requiresAuth: true,
              titleKey: 'routes.commands',
            },
          },
          {
            path: '/tasks',
            name: 'tasks',
            component: () => import('@/views/operations/TasksView.vue'),
            meta: {
              icon: 'tasks',
              keepAlive: true,
              requiresAuth: true,
              titleKey: 'routes.tasks',
            },
          },
          {
            path: '/logs',
            name: 'logs',
            component: () => import('@/views/operations/LogsView.vue'),
            meta: {
              icon: 'logs',
              keepAlive: true,
              requiresAuth: true,
              titleKey: 'routes.logs',
            },
          },
        ],
      },
      {
        path: '',
        component: RouteView,
        redirect: { name: 'protocols' },
        meta: {
          hideInTab: true,
          icon: 'protocols',
          order: 4,
          requiresAuth: true,
          titleKey: 'routes.protocolGroup',
        },
        children: [
          {
            path: '/protocols',
            name: 'protocols',
            component: () => import('@/views/protocols/ProtocolsView.vue'),
            meta: {
              icon: 'protocols',
              keepAlive: true,
              requiresAuth: true,
              titleKey: 'routes.protocols',
            },
          },
          {
            path: '/protocols/logs',
            name: 'protocol-logs',
            component: () => import('@/views/protocols/ProtocolLogsView.vue'),
            meta: {
              icon: 'protocol-logs',
              keepAlive: true,
              requiresAuth: true,
              titleKey: 'routes.protocolLogs',
            },
          },
        ],
      },
      {
        path: '',
        component: RouteView,
        redirect: { name: 'config' },
        meta: {
          hideInTab: true,
          icon: 'system',
          order: 5,
          requiresAuth: true,
          titleKey: 'routes.system',
        },
        children: [
          {
            path: '/config',
            name: 'config',
            component: () => import('@/views/system/ConfigView.vue'),
            meta: {
              icon: 'config',
              keepAlive: true,
              requiresAuth: true,
              titleKey: 'routes.config',
            },
          },
        ],
      },
    ],
  },
]
