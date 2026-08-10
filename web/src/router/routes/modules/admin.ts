import type { RouteRecordRaw } from 'vue-router'

import BasicLayout from '@/layouts/BasicLayout.vue'
import RouteView from '@/layouts/RouteView.vue'
import ExceptionView from '@/views/core/ExceptionView.vue'

function groupRoute(
  titleKey: string,
  icon: string,
  order: number,
  redirectName: string,
  children: RouteRecordRaw[],
): RouteRecordRaw {
  return {
    path: '',
    component: RouteView,
    redirect: { name: redirectName },
    meta: {
      hideInTab: true,
      icon,
      order,
      requiresAuth: true,
      titleKey,
    },
    children,
  }
}

function exceptionRoute(
  path: string,
  name: string,
  status: '403' | '404' | '500' | 'offline',
  titleKey: string,
  options: { public?: boolean; hideInTab?: boolean } = {},
): RouteRecordRaw {
  return {
    path,
    name,
    component: ExceptionView,
    props: { status },
    meta: {
      exceptionStatus: status,
      hideInMenu: true,
      hideInTab: options.hideInTab,
      icon: status,
      public: options.public,
      requiresAuth: !options.public,
      titleKey,
      viewKey: name,
    },
  }
}

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
      groupRoute('routes.features', 'features', 2, 'menu-center', [
        {
          path: '/menu-center',
          name: 'menu-center',
          component: () => import('@/views/builtin/MenuCenterView.vue'),
          meta: {
            icon: 'menu-center',
            keepAlive: true,
            order: 1,
            requiresAuth: true,
            titleKey: 'routes.menuCenter',
            viewKey: 'menu-center',
          },
        },
        {
          path: '/plugins/store',
          name: 'plugin-store',
          component: () => import('@/views/plugins/PluginStoreView.vue'),
          meta: {
            icon: 'plugin-store',
            keepAlive: true,
            order: 2,
            requiresAuth: true,
            titleKey: 'routes.pluginStore',
            viewKey: 'plugin-store',
          },
        },
        {
          path: '/plugins',
          name: 'plugins',
          component: () => import('@/views/plugins/PluginsView.vue'),
          meta: {
            icon: 'plugins',
            keepAlive: true,
            order: 3,
            requiresAuth: true,
            titleKey: 'routes.pluginList',
          },
        },
        {
          path: '/plugins/settings',
          name: 'plugin-settings',
          component: () => import('@/views/plugins/PluginSettingsView.vue'),
          meta: {
            icon: 'plugin-settings',
            keepAlive: true,
            order: 4,
            requiresAuth: true,
            titleKey: 'routes.pluginSettings',
            viewKey: 'plugin-settings',
          },
        },
        {
          path: '/commands',
          name: 'commands',
          component: () => import('@/views/operations/CommandsView.vue'),
          meta: {
            icon: 'commands',
            keepAlive: true,
            order: 5,
            requiresAuth: true,
            titleKey: 'routes.commands',
            viewKey: 'commands',
          },
        },
        {
          path: '/plugins/:id',
          name: 'plugin-detail',
          component: () => import('@/views/plugins/PluginDetailView.vue'),
          meta: {
            activePath: '/plugins',
            hideInMenu: true,
            requiresAuth: true,
            titleKey: 'routes.pluginDetail',
          },
        },
      ]),
      groupRoute('routes.connections', 'connections', 3, 'third-party-accounts', [
        {
          path: '/third-party-accounts',
          name: 'third-party-accounts',
          component: () => import('@/views/builtin/ThirdPartyAccountsView.vue'),
          meta: {
            icon: 'third-party-accounts',
            keepAlive: true,
            order: 1,
            requiresAuth: true,
            titleKey: 'routes.thirdPartyAccounts',
            viewKey: 'third-party-accounts',
          },
        },
        {
          path: '/protocols',
          name: 'protocols',
          component: () => import('@/views/protocols/ProtocolsView.vue'),
          meta: {
            icon: 'protocols',
            keepAlive: true,
            order: 2,
            requiresAuth: true,
            titleKey: 'routes.protocols',
          },
        },
        {
          path: '/protocols/compatibility',
          name: 'protocols-compatibility',
          component: () => import('@/views/protocols/ProtocolCompatibilityView.vue'),
          meta: {
            icon: 'protocol-compatibility',
            keepAlive: true,
            order: 3,
            requiresAuth: true,
            titleKey: 'routes.protocolCompatibility',
          },
        },
      ]),
      groupRoute('routes.governance', 'toolbox', 4, 'permission-policy', [
        {
          path: '/permission-policy',
          name: 'permission-policy',
          component: () => import('@/views/operations/PermissionPolicyView.vue'),
          meta: {
            icon: 'permission-policy',
            keepAlive: true,
            order: 1,
            requiresAuth: true,
            titleKey: 'routes.permissionPolicy',
            viewKey: 'permission-policy',
          },
        },
        {
          path: '/access-lists',
          name: 'access-lists',
          component: () => import('@/views/operations/AccessListsView.vue'),
          meta: {
            icon: 'access-lists',
            keepAlive: true,
            order: 2,
            requiresAuth: true,
            titleKey: 'routes.accessLists',
            viewKey: 'access-lists',
          },
        },
        {
          path: '/rate-limits',
          name: 'rate-limits',
          component: () => import('@/views/operations/RateLimitsView.vue'),
          meta: {
            icon: 'rate-limits',
            keepAlive: true,
            order: 3,
            requiresAuth: true,
            titleKey: 'routes.rateLimits',
            viewKey: 'rate-limits',
          },
        },
      ]),
      groupRoute('routes.runtime', 'runtime', 5, 'scheduler', [
        {
          path: '/scheduler',
          name: 'scheduler',
          component: () => import('@/views/operations/SchedulerJobsView.vue'),
          meta: {
            icon: 'scheduler',
            keepAlive: true,
            order: 1,
            requiresAuth: true,
            titleKey: 'routes.scheduler',
            viewKey: 'scheduler',
          },
        },
        {
          path: '/logs',
          name: 'logs',
          component: () => import('@/views/operations/LogsView.vue'),
          meta: {
            icon: 'logs',
            keepAlive: true,
            order: 2,
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
            order: 3,
            requiresAuth: true,
            titleKey: 'routes.logsHistory',
            viewKey: 'logs-history',
          },
        },
      ]),
      groupRoute('routes.system', 'system', 6, 'config', [
        {
          path: '/config',
          name: 'config',
          component: () => import('@/views/system/ConfigView.vue'),
          meta: {
            icon: 'config',
            keepAlive: true,
            order: 1,
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
            order: 2,
            requiresAuth: true,
            titleKey: 'routes.renderTemplates',
            viewKey: 'render-templates',
          },
        },
      ]),
      exceptionRoute('/403', 'forbidden', '403', 'routes.forbidden'),
      exceptionRoute('/404', 'not-found-page', '404', 'routes.notFound'),
      exceptionRoute('/500', 'server-error', '500', 'routes.serverError'),
      exceptionRoute('/offline', 'offline', 'offline', 'routes.offline', { public: true, hideInTab: true }),
      {
        path: '/:pathMatch(.*)*',
        name: 'not-found',
        component: ExceptionView,
        props: { status: '404' },
        meta: {
          exceptionStatus: '404',
          hideInMenu: true,
          icon: '404',
          requiresAuth: true,
          titleKey: 'routes.notFound',
        },
      },
    ],
  },
]
