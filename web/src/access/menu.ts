import type { RouteRecordRaw } from 'vue-router'

import { t } from '@/i18n'

declare module 'vue-router' {
  interface RouteMeta {
    activePath?: string
    affixTab?: boolean
    entryPath?: string
    hideInMenu?: boolean
    hideInTab?: boolean
    icon?: string
    keepAlive?: boolean
    order?: number
    title?: string
    titleKey?: string
    viewKey?: string
  }
}

export interface AppMenuItem {
  children?: AppMenuItem[]
  icon?: string
  key: string
  path: string
  title: string
}

export interface AppNavigationItem {
  icon?: string
  key: string
  path: string
  title: string
}

function joinRoutePath(parentPath: string, childPath: string) {
  if (!childPath) {
    return parentPath || '/'
  }

  if (childPath.startsWith('/')) {
    return childPath
  }

  const prefix = parentPath === '/' ? '' : parentPath
  return `${prefix}/${childPath}` || '/'
}

export function resolveRouteTitle(meta?: Record<string, unknown> | null) {
  if (!meta) {
    return ''
  }

  if (typeof meta.titleKey === 'string' && meta.titleKey) {
    return t(meta.titleKey)
  }

  return typeof meta.title === 'string' ? meta.title : ''
}

export function resolveRouteEntryPath(meta: Record<string, unknown> | null | undefined, fallbackPath: string) {
  if (typeof meta?.entryPath === 'string' && meta.entryPath) {
    return meta.entryPath
  }

  return fallbackPath
}

export function buildMenuItems(routes: RouteRecordRaw[], parentPath = ''): AppMenuItem[] {
  return routes
    .filter((route) => !route.meta?.hideInMenu)
    .map((route) => {
      const routePath = joinRoutePath(parentPath, route.path)
      const path = resolveRouteEntryPath(route.meta, routePath)
      const children = route.children ? buildMenuItems(route.children, routePath) : []
      const title = resolveRouteTitle(route.meta)

      return {
        children: children.length > 0 ? children : undefined,
        icon: route.meta?.icon,
        key: String(route.name ?? `menu:${path}:${title}`),
        path,
        title,
        order: route.meta?.order ?? 0,
      } as AppMenuItem & { order: number }
    })
    .filter((route) => route.title)
    .sort((left, right) => left.order - right.order)
    .map(({ order: _order, ...route }) => route)
}

export function collectNavigationItems(routes: RouteRecordRaw[], parentPath = ''): AppNavigationItem[] {
  return routes.flatMap((route) => {
    const routePath = joinRoutePath(parentPath, route.path)
    const path = resolveRouteEntryPath(route.meta, routePath)
    const title = resolveRouteTitle(route.meta)
    const children = route.children ? collectNavigationItems(route.children, routePath) : []
    const current = title && !route.meta?.hideInMenu
      ? [{
        icon: route.meta?.icon,
        key: String(route.name ?? `nav:${path}:${title}`),
        path,
        title,
      }]
      : []

    return [...current, ...children]
  })
}
