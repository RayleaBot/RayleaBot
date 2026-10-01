import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, type RouteLocationRaw } from 'vue-router'

import { buildMenuItems, collectNavigationItems, type AppMenuItem, type AppNavigationItem } from '@/access/menu'
import { isPluginCenterRoute, pluginCenterPages, pluginCenterPath, projectPluginCenterMenu } from '@/access/plugin-center'
import { adminRoutes } from '@/router/routes/modules/admin'
import { useUiShellStore } from '@/stores/ui-shell'
import { t } from '@/i18n'
import { getLeafRouteMeta } from './shell-routes'

export type PluginNavigationScope = 'root' | 'plugin-center'

interface MenuLineageEntry {
  key: string
  path: string
}

function flattenMenu(items: AppMenuItem[], lineage: MenuLineageEntry[] = []): Array<{ item: AppMenuItem; lineage: MenuLineageEntry[] }> {
  return items.flatMap((item) => {
    const currentLineage = [...lineage, { key: item.key, path: item.path }]
    const current = [{ item, lineage: currentLineage }]
    return item.children ? [...current, ...flattenMenu(item.children, currentLineage)] : current
  })
}

// Arrow, Home and End move focus between visible, enabled navigation items.
export function handleNavigationKeydown(event: KeyboardEvent) {
  if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return

  const container = event.currentTarget
  if (!(container instanceof HTMLElement)) return

  const items = Array.from(container.querySelectorAll<HTMLElement>('[data-nav-item]'))
    .filter((item) => item.getClientRects().length > 0 && item.getAttribute('aria-disabled') !== 'true')
  if (items.length === 0) return

  const activeIndex = items.findIndex((item) => item === document.activeElement || item.contains(document.activeElement))
  let nextIndex = 0
  if (event.key === 'End') {
    nextIndex = items.length - 1
  } else if (event.key === 'ArrowUp') {
    nextIndex = activeIndex <= 0 ? items.length - 1 : activeIndex - 1
  } else if (event.key === 'ArrowDown' && activeIndex >= 0) {
    nextIndex = (activeIndex + 1) % items.length
  }

  event.preventDefault()
  items[nextIndex]?.focus()
}

export function useShellNavigation(options: {
  navigate: (target: RouteLocationRaw) => unknown
}) {
  const route = useRoute()
  const uiShellStore = useUiShellStore()
  const { siderCollapsed } = storeToRefs(uiShellStore)
  // Read at setup: the admin route module imports the layout that uses this composable.
  const adminPageRoutes = adminRoutes[0]?.children ?? []

  const menuItems = computed(() => projectPluginCenterMenu(buildMenuItems(adminPageRoutes, '')))
  const flattenedMenu = flattenMenu(menuItems.value)
  const openMenuKeys = ref<string[]>(menuItems.value.filter(item => item.children?.length).map(item => item.key))
  const collapsedOpenMenuKeys = ref<string[]>([])
  watch(siderCollapsed, () => { collapsedOpenMenuKeys.value = [] })

  // Plugin detail and plugin center pages belong to the plugin center menu entry.
  const menuLineage = computed(() => {
    const leafMeta = getLeafRouteMeta(route)
    const targetPath = isPluginCenterRoute(route.name) || route.name === 'plugin-detail'
      ? pluginCenterPath
      : typeof leafMeta?.activePath === 'string' && leafMeta.activePath
        ? leafMeta.activePath
        : route.path
    return flattenedMenu.find(({ item }) => item.path === targetPath)?.lineage ?? []
  })
  const selectedMenuKeys = computed(() => {
    const last = menuLineage.value.at(-1)
    return last ? [last.key] : []
  })
  watch(menuLineage, (lineage) => {
    const routeKeys = lineage.slice(0, -1).map((item) => item.key)
    openMenuKeys.value = Array.from(new Set([...openMenuKeys.value, ...routeKeys]))
  }, { immediate: true })

  const pluginNavigationScope = ref<PluginNavigationScope>('root')
  watch(
    [() => route.name, () => route.params.id],
    ([routeName, routePluginId]) => {
      const pluginDetail = routeName === 'plugin-detail' && typeof routePluginId === 'string' && Boolean(routePluginId)
      pluginNavigationScope.value = pluginDetail || isPluginCenterRoute(routeName) ? 'plugin-center' : 'root'
    },
    { immediate: true },
  )

  // Search lists every page once per path, in sidebar order; plugin center pages follow their own sidebar layer.
  const pluginCenterRank = new Map<string, number>(pluginCenterPages.map((page, index) => [page.path, index]))
  const collectedItems = collectNavigationItems(adminPageRoutes, '')
    .filter((item, index, items) => items.findIndex(other => other.path === item.path) === index)
  const pluginCenterItems = collectedItems
    .filter(item => pluginCenterRank.has(item.path))
    .sort((left, right) => (pluginCenterRank.get(left.path) ?? 0) - (pluginCenterRank.get(right.path) ?? 0))
  let pluginCenterSlot = 0
  // The sidebar shows the plugin center pages under 插件中心, so search names that section too.
  const navigationItems: AppNavigationItem[] = collectedItems
    .map(item => (pluginCenterRank.has(item.path) ? pluginCenterItems[pluginCenterSlot++] ?? item : item))
    .map(item => (pluginCenterRank.has(item.path) ? { ...item, section: t('routes.pluginCenter') } : item))

  function navigateTo(target: RouteLocationRaw) {
    collapsedOpenMenuKeys.value = []
    void options.navigate(target)
  }

  function handleOpenChange(keys: string[]) {
    if (siderCollapsed.value) collapsedOpenMenuKeys.value = keys.slice(-1)
    else openMenuKeys.value = keys
  }

  function setPluginNavigationScope(scope: PluginNavigationScope) {
    pluginNavigationScope.value = scope
  }

  return {
    collapsedOpenMenuKeys,
    handleOpenChange,
    menuItems,
    navigateTo,
    navigationItems,
    openMenuKeys,
    pluginNavigationScope,
    selectedMenuKeys,
    setPluginNavigationScope,
  }
}
