import type { RouteRecordRaw } from 'vue-router'

import AuthLayout from '@/layouts/AuthLayout.vue'

export const publicRoutes: RouteRecordRaw[] = [
  {
    path: '/auth',
    component: AuthLayout,
    meta: { public: true },
    children: [
      {
        path: '/login',
        name: 'login',
        component: () => import('@/views/auth/LoginView.vue'),
        meta: {
          public: true,
          titleKey: 'routes.login',
          icon: 'login',
          hideInMenu: true,
          hideInTab: true,
        },
      },
      {
        path: '/setup',
        name: 'setup',
        component: () => import('@/views/auth/SetupView.vue'),
        meta: {
          public: true,
          titleKey: 'routes.setup',
          icon: 'login',
          hideInMenu: true,
          hideInTab: true,
        },
      },
    ],
  },
]
