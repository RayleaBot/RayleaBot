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
        redirect: { name: 'governance' },
        meta: {
          hideInTab: true,
          icon: 'toolbox',
          order: 3,
          requiresAuth: true,
          titleKey: 'routes.operations',
        },
        children: [
          {
            path: '/governance',
            name: 'governance',
            component: () => import('@/views/operations/GovernanceView.vue'),
            meta: {
              icon: 'governance',
              keepAlive: true,
              requiresAuth: true,
              titleKey: 'routes.governance',
              viewKey: 'governance',
            },
          },
          {
            path: '/commands',
            name: 'commands',
            component: () => import('@/views/operations/CommandsView.vue'),
            meta: {
              icon: 'commands',
              keepAlive: true,
              requiresAuth: true,
              titleKey: 'routes.commands',
              viewKey: 'commands',
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
              viewKey: 'tasks',
            },
          },
        ],
      },
      {
        path: '',
        component: RouteView,
        redirect: { name: 'logs' },
        meta: {
          hideInTab: true,
          icon: 'logs-center',
          order: 4,
          requiresAuth: true,
          titleKey: 'routes.logsCenter',
        },
        children: [
          {
            path: '/logs',
            name: 'logs',
            component: () => import('@/views/operations/LogsView.vue'),
            meta: {
              icon: 'logs',
              keepAlive: true,
              requiresAuth: true,
              titleKey: 'routes.logs',
              viewKey: 'logs',
            },
          },
          {
            path: '/logs/history',
            name: 'logs-history',
            component: () => import('@/views/operations/LogsHistoryView.vue'),
            meta: {
              icon: 'history-logs',
              keepAlive: true,
              requiresAuth: true,
              titleKey: 'routes.logsHistory',
              viewKey: 'logs-history',
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
          order: 5,
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
            path: '/protocols/compatibility',
            name: 'protocols-compatibility',
            component: () => import('@/views/protocols/ProtocolCompatibilityView.vue'),
            meta: {
              icon: 'protocols',
              keepAlive: true,
              requiresAuth: true,
              titleKey: 'routes.protocolCompatibility',
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
          order: 6,
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
          {
            path: '/render/templates/:templateId?',
            name: 'render-templates',
            component: () => import('@/views/system/RenderTemplatesView.vue'),
            meta: {
              activePath: '/render/templates',
              entryPath: '/render/templates',
              icon: 'render-templates',
              keepAlive: true,
              requiresAuth: true,
              titleKey: 'routes.renderTemplates',
              viewKey: 'render-templates',
            },
          },
        ],
      },
    ],
  },
]
