import { computed, nextTick, onBeforeUnmount, onMounted, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter, type RouteLocationNormalizedLoaded, type RouteLocationRaw } from 'vue-router'

import { resolveMenuIcon } from '@/access/icons'
import { resolveRouteEntryPath, resolveRouteTitle } from '@/access/menu'
import { createPluginCenterTab, isPluginCenterRoute, pluginCenterPath } from '@/access/plugin-center'
import { t } from '@/i18n'
import { adminRoutes } from '@/router/routes/modules/admin'
import { usePluginsStore } from '@/stores/plugins'
import { useUiShellStore, type ShellTabItem } from '@/stores/ui-shell'
import { collectAffixTabs, getLeafRouteMeta, resolveRouteIcon } from './shell-routes'

export type TabActionKey = 'close-current' | 'close-other' | 'close-left' | 'close-right' | 'close-all'

export interface TabActionItem {
  disabled?: boolean
  key: TabActionKey
  label: string
}

interface WorkspaceTabProjection {
  replaceByName?: string
  tab: ShellTabItem
}

export function useWorkspaceTabs(navigate: (target: RouteLocationRaw) => unknown) {
  const route = useRoute()
  const router = useRouter()
  const uiShellStore = useUiShellStore()
  const pluginsStore = usePluginsStore()
  const { tabs } = storeToRefs(uiShellStore)

  const affixTabs = collectAffixTabs(adminRoutes)
  uiShellStore.syncTabs(affixTabs)

  function resolveTabPath(viewRoute: RouteLocationNormalizedLoaded) {
    if (isPluginCenterRoute(viewRoute.name)) return pluginCenterPath
    return resolveRouteEntryPath(getLeafRouteMeta(viewRoute), viewRoute.path)
  }

  function resolveTabTitle(viewRoute: RouteLocationNormalizedLoaded) {
    const pluginId = viewRoute.params.id
    if (viewRoute.name === 'plugin-detail' && typeof pluginId === 'string' && pluginId) {
      return t('plugins.detailPageTitle', { name: pluginsStore.getPluginDisplayName(pluginId) })
    }
    return resolveRouteTitle(getLeafRouteMeta(viewRoute))
  }

  function resolveCurrentTab(viewRoute: RouteLocationNormalizedLoaded): WorkspaceTabProjection | null {
    const leafMeta = getLeafRouteMeta(viewRoute)
    if (!viewRoute.matched.length || leafMeta?.hideInTab || !viewRoute.name) return null
    if (isPluginCenterRoute(viewRoute.name)) return { tab: createPluginCenterTab(viewRoute.fullPath) }

    const title = resolveTabTitle(viewRoute)
    if (!title) return null

    return {
      // Query-driven workspaces keep a single tab per page.
      replaceByName: typeof leafMeta?.viewKey === 'string' && leafMeta.viewKey ? String(viewRoute.name) : undefined,
      tab: {
        affix: Boolean(leafMeta?.affixTab),
        fullPath: viewRoute.fullPath,
        icon: resolveRouteIcon(router, viewRoute),
        keepAlive: Boolean(leafMeta?.keepAlive),
        name: String(viewRoute.name),
        path: resolveTabPath(viewRoute),
        title,
      },
    }
  }

  function resolveTabPluginId(item: ShellTabItem) {
    if (item.name !== 'plugin-detail') return null
    try {
      const pluginId = router.resolve(item.fullPath).params.id
      return typeof pluginId === 'string' && pluginId ? pluginId : null
    } catch {
      return null
    }
  }

  function resolveTabPlugin(item: ShellTabItem) {
    const pluginId = resolveTabPluginId(item)
    if (!pluginId) return null
    const plugin = pluginsStore.detailsByPluginId[pluginId] ?? pluginsStore.knownItems.find(candidate => candidate.id === pluginId)
    return { id: pluginId, icon: plugin?.icon, version: plugin?.version }
  }

  // Current route metadata wins over the icon stored with a restored tab.
  function resolveTabIconName(item: ShellTabItem) {
    try {
      return resolveRouteIcon(router, router.resolve(item.path)) || item.icon
    } catch {
      return item.icon
    }
  }

  const tabViews = computed(() => tabs.value.map((tab) => {
    const plugin = resolveTabPlugin(tab)
    const iconName = resolveTabIconName(tab)
    return {
      tab,
      plugin,
      iconName,
      icon: resolveMenuIcon(iconName),
      iconData: plugin ? `plugin:${plugin.id}` : iconName || undefined,
    }
  }))

  const openPluginTargets = computed(() => {
    const seen = new Set<string>()
    return tabs.value.flatMap((tab) => {
      const pluginId = resolveTabPluginId(tab)
      if (!pluginId || seen.has(pluginId)) return []
      seen.add(pluginId)
      return [{ fullPath: tab.fullPath, pluginId }]
    })
  })

  // Open plugin tabs need the plugin list for their names and icons.
  watch(() => openPluginTargets.value.length, (count) => {
    if (count > 0) void pluginsStore.ensureList().catch(() => undefined)
  }, { immediate: true })

  const currentTabPath = computed(() => resolveTabPath(route))
  const currentTab = computed(() => tabs.value.find((item) => item.path === currentTabPath.value) ?? null)

  watch(
    () => route.fullPath,
    () => {
      nextTick(() => {
        const projection = resolveCurrentTab(route)
        if (!projection) {
          return
        }

        if (projection.replaceByName) {
          uiShellStore.removeTabsByName(projection.replaceByName, { exceptPath: projection.tab.path })
        }

        uiShellStore.upsertTab(projection.tab)
        uiShellStore.setMobileMenuOpen(false)
      })
    },
    { immediate: true },
  )

  // Plugin detail tabs are titled by the plugin name, which may arrive after navigation.
  watch(
    () => route.name === 'plugin-detail' ? resolveTabTitle(route) : null,
    () => {
      if (route.name !== 'plugin-detail') return
      const projection = resolveCurrentTab(route)
      if (projection) uiShellStore.upsertTab(projection.tab)
    },
  )

  function onTabChange(targetKey: string) {
    const targetTab = tabs.value.find((item) => item.path === targetKey)
    void navigate(targetTab?.fullPath ?? targetKey)
  }

  function findTab(path: string) {
    return tabs.value.find((item) => item.path === path) ?? null
  }

  function getFallbackTab(targetPath: string, beforeTabs: ShellTabItem[], afterTabs: ShellTabItem[]) {
    const targetTab = afterTabs.find((item) => item.path === targetPath)
    if (targetTab) {
      return targetTab
    }

    const targetIndex = beforeTabs.findIndex((item) => item.path === targetPath)
    if (targetIndex >= 0) {
      const leftTab = beforeTabs
        .slice(0, targetIndex)
        .reverse()
        .find((item) => afterTabs.some((candidate) => candidate.path === item.path))
      if (leftTab) {
        return afterTabs.find((item) => item.path === leftTab.path) ?? leftTab
      }

      const rightTab = beforeTabs
        .slice(targetIndex + 1)
        .find((item) => afterTabs.some((candidate) => candidate.path === item.path))
      if (rightTab) {
        return afterTabs.find((item) => item.path === rightTab.path) ?? rightTab
      }
    }

    return afterTabs[0] ?? affixTabs[0] ?? null
  }

  function closeTabsWithFallback(targetPath: string, mutateTabs: () => void) {
    const beforeTabs = [...tabs.value]
    const activePathBefore = currentTabPath.value

    mutateTabs()

    if (tabs.value.some((item) => item.path === activePathBefore)) {
      return
    }

    const fallback = getFallbackTab(targetPath, beforeTabs, tabs.value)
    if (fallback) {
      void navigate(fallback.fullPath)
    }
  }

  function closeTab(targetPath: string) {
    const targetTab = findTab(targetPath)
    if (!targetTab || targetTab.affix) {
      return
    }

    closeTabsWithFallback(targetPath, () => uiShellStore.removeTab(targetPath))
  }

  function onTabEdit(targetKey: string | MouseEvent, action: 'add' | 'remove') {
    if (action === 'remove' && typeof targetKey === 'string') {
      closeTab(targetKey)
    }
  }

  function closeOtherTabs(targetPath = currentTabPath.value) {
    if (findTab(targetPath)) {
      closeTabsWithFallback(targetPath, () => uiShellStore.closeOtherTabs(targetPath))
    }
  }

  function closeTabsToLeft(targetPath = currentTabPath.value) {
    if (findTab(targetPath)) {
      closeTabsWithFallback(targetPath, () => uiShellStore.closeTabsToLeft(targetPath))
    }
  }

  function closeTabsToRight(targetPath = currentTabPath.value) {
    if (findTab(targetPath)) {
      closeTabsWithFallback(targetPath, () => uiShellStore.closeTabsToRight(targetPath))
    }
  }

  function closeAllTabs(targetPath = currentTabPath.value) {
    closeTabsWithFallback(targetPath, () => uiShellStore.closeAllTabs())
  }

  function handleTabAction(key: string | number, targetTab = currentTab.value) {
    const actionKey = String(key) as TabActionKey
    switch (actionKey) {
      case 'close-current':
        if (targetTab && !targetTab.affix) closeTab(targetTab.path)
        return
      case 'close-other':
        if (targetTab) closeOtherTabs(targetTab.path)
        return
      case 'close-left':
        if (targetTab) closeTabsToLeft(targetTab.path)
        return
      case 'close-right':
        if (targetTab) closeTabsToRight(targetTab.path)
        return
      case 'close-all':
        closeAllTabs(targetTab?.path ?? currentTabPath.value)
    }
  }

  function hasClosableTabsBefore(index: number) {
    return tabs.value.slice(0, index).some((item) => !item.affix)
  }

  function hasClosableTabsAfter(index: number) {
    return tabs.value.slice(index + 1).some((item) => !item.affix)
  }

  function getTabCloseActionItems(targetTab: ShellTabItem | null | undefined): TabActionItem[] {
    const targetIndex = targetTab ? tabs.value.findIndex((item) => item.path === targetTab.path) : -1
    const hasClosableTabs = tabs.value.some((item) => !item.affix)

    return [
      { disabled: !targetTab || targetTab.affix, key: 'close-current', label: t('shell.tabActions.closeCurrent') },
      { disabled: !targetTab || !tabs.value.some((item) => !item.affix && item.path !== targetTab.path), key: 'close-other', label: t('shell.tabActions.closeOther') },
      { disabled: targetIndex < 0 || !hasClosableTabsBefore(targetIndex), key: 'close-left', label: t('shell.tabActions.closeLeft') },
      { disabled: targetIndex < 0 || !hasClosableTabsAfter(targetIndex), key: 'close-right', label: t('shell.tabActions.closeRight') },
      { disabled: !hasClosableTabs, key: 'close-all', label: t('shell.tabActions.closeAll') },
    ]
  }

  const tabActionItems = computed<TabActionItem[]>(() => getTabCloseActionItems(currentTab.value))

  function isEditableTarget(target: EventTarget | null) {
    return target instanceof HTMLElement
      && (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName))
  }

  function handleGlobalShortcut(event: KeyboardEvent) {
    const targetIsEditable = isEditableTarget(event.target)
    const commandKey = event.metaKey || event.ctrlKey

    if (commandKey && event.key.toLowerCase() === 'k') {
      event.preventDefault()
      uiShellStore.openSearch()
      return
    }

    if (event.altKey && event.shiftKey && event.key.toLowerCase() === 's') {
      event.preventDefault()
      uiShellStore.openSettings()
      return
    }

    if (targetIsEditable) {
      return
    }

    if (commandKey && event.shiftKey && event.key.toLowerCase() === 'w') {
      event.preventDefault()
      closeOtherTabs()
      return
    }

    if (commandKey && event.key.toLowerCase() === 'w' && currentTab.value && !currentTab.value.affix) {
      event.preventDefault()
      closeTab(currentTab.value.path)
    }
  }

  onMounted(() => {
    if (typeof document !== 'undefined') {
      document.addEventListener('keydown', handleGlobalShortcut)
    }
  })

  onBeforeUnmount(() => {
    if (typeof document !== 'undefined') {
      document.removeEventListener('keydown', handleGlobalShortcut)
    }
  })

  return {
    currentTabPath,
    getTabCloseActionItems,
    handleTabAction,
    onTabChange,
    onTabEdit,
    openPluginTargets,
    tabActionItems,
    tabViews,
  }
}
