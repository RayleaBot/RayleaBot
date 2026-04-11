import type { RouteLocationMatched, RouteRecordRaw } from 'vue-router'

import { t } from '@/i18n'

declare module 'vue-router' {
  interface RouteMeta {
    activePath?: string
    affixTab?: boolean
    hideInMenu?: boolean
    hideInTab?: boolean
    icon?: string
    keepAlive?: boolean
    order?: number
    title?: string
    titleKey?: string
  }
}

export interface AppMenuItem {
  children?: AppMenuItem[]
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

export function buildMenuItems(routes: RouteRecordRaw[], parentPath = ''): AppMenuItem[] {
  return routes
    .filter((route) => !route.meta?.hideInMenu)
    .map((route) => {
      const path = joinRoutePath(parentPath, route.path)
      const children = route.children ? buildMenuItems(route.children, path) : []
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

export function getMatchedBreadcrumbs(matched: RouteLocationMatched[]) {
  const seen = new Set<string>()

  return matched
    .map((record) => ({
      path: record.path,
      title: resolveRouteTitle(record.meta),
    }))
    .filter((item) => {
      if (!item.title) {
        return false
      }

      const key = `${item.path}:${item.title}`
      if (seen.has(key)) {
        return false
      }

      seen.add(key)
      return true
    })
}
