import { defineComponent, h, markRaw, resolveDynamicComponent, type Component } from 'vue'
import type { RouteLocationNormalizedLoaded, Router } from 'vue-router'

type MatchedRoute = Pick<RouteLocationNormalizedLoaded, 'matched'>
type ViewRoute = Pick<RouteLocationNormalizedLoaded, 'matched' | 'name' | 'path'>

export function getLeafRouteMeta(route: MatchedRoute) {
  return route.matched.at(-1)?.meta ?? null
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

// Pages marked keepAlive stay cached after their first visit, so drafts, filters and scroll positions
// survive switching pages. KeepAlive matches the route stage components by route name.
export function collectKeepAliveViewNames(router: Router) {
  return router.getRoutes()
    .filter(record => record.meta.keepAlive && record.name)
    .map(record => String(record.name))
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
