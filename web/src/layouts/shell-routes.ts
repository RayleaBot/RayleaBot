import { defineComponent, h, markRaw, resolveDynamicComponent, type Component } from 'vue'
import type { RouteLocationNormalizedLoaded, RouteRecordRaw, Router } from 'vue-router'

import { joinRoutePath, resolveRouteEntryPath, resolveRouteTitle } from '@/access/menu'
import type { ShellTabItem } from '@/stores/ui-shell'

type MatchedRoute = Pick<RouteLocationNormalizedLoaded, 'matched'>
type ViewRoute = Pick<RouteLocationNormalizedLoaded, 'matched' | 'name' | 'path'>

export function getLeafRouteMeta(route: MatchedRoute) {
  return route.matched.at(-1)?.meta ?? null
}

export function resolveRouteIconName(meta?: Record<string, unknown> | null) {
  return typeof meta?.icon === 'string' && meta.icon ? meta.icon : undefined
}

// Pages without their own icon use the icon of the page they are shown under.
export function resolveRouteIcon(router: Router, route: MatchedRoute) {
  const leafMeta = getLeafRouteMeta(route)
  const directIcon = resolveRouteIconName(leafMeta)
  if (directIcon) return directIcon

  const activePath = typeof leafMeta?.activePath === 'string' && leafMeta.activePath ? leafMeta.activePath : null
  if (!activePath) return undefined

  try {
    return resolveRouteIconName(router.resolve(activePath).matched.at(-1)?.meta ?? null)
  } catch {
    return undefined
  }
}

// Query-driven workspaces share one cached view per viewKey; other pages get one per path.
export function resolveRouteViewKey(route: ViewRoute) {
  const viewKey = getLeafRouteMeta(route)?.viewKey
  if (typeof viewKey === 'string' && viewKey) return viewKey
  return `${String(route.name ?? route.path)}:${route.path}`
}

export function resolveLeafRouteComponent(route: RouteLocationNormalizedLoaded) {
  return route.matched.at(-1)?.components?.default ?? null
}

export function collectAffixTabs(routes: RouteRecordRaw[], parentPath = ''): ShellTabItem[] {
  return routes.flatMap((item) => {
    const routePath = joinRoutePath(parentPath, item.path)
    const path = resolveRouteEntryPath(item.meta, routePath)
    const title = resolveRouteTitle(item.meta)
    const children = item.children ? collectAffixTabs(item.children, routePath) : []
    const current = item.meta?.affixTab && title && item.name
      ? [{
          affix: true,
          fullPath: path,
          icon: resolveRouteIconName(item.meta),
          keepAlive: Boolean(item.meta?.keepAlive),
          name: String(item.name),
          path,
          title,
        }]
      : []

    return [...current, ...children]
  })
}

// KeepAlive matches cached views by component name, so every route renders inside a stage
// component named after the route.
export function createRouteStageRegistry() {
  const stages = new Map<string, Component>()

  return (route: RouteLocationNormalizedLoaded) => {
    const stageName = String(route.name ?? route.path)
    const cached = stages.get(stageName)
    if (cached) return cached

    const stage = markRaw(defineComponent({
      name: stageName,
      props: {
        routeComponent: {
          required: true,
          type: [Function, Object, String],
        },
      },
      setup(props) {
        const routeComponent = props.routeComponent
        return () => h('div', { class: 'admin-layout__route-stage' }, [h(resolveDynamicComponent(routeComponent) as Component)])
      },
    }))
    stages.set(stageName, stage)
    return stage
  }
}
