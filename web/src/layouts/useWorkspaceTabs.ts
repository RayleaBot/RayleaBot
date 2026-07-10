import { computed, nextTick, onBeforeUnmount, onMounted, watch, type Ref } from 'vue'
import type { RouteLocationNormalizedLoaded, Router } from 'vue-router'

import { t } from '@/i18n'
import { useUiShellStore, type ShellTabItem } from '@/stores/ui-shell'

export type TabActionKey = 'close-current' | 'close-other' | 'close-left' | 'close-right' | 'close-all'

export interface TabActionItem {
  disabled?: boolean
  key: TabActionKey
  label: string
}

export interface WorkspaceTabProjection {
  replaceByName?: string
  tab: ShellTabItem
}

interface WorkspaceTabsOptions {
  affixTabs: ShellTabItem[]
  resolveCurrentTab: (route: RouteLocationNormalizedLoaded) => WorkspaceTabProjection | null
  resolveTabPath: (route: RouteLocationNormalizedLoaded) => string
  route: RouteLocationNormalizedLoaded
  router: Router
  tabs: Ref<ShellTabItem[]>
  uiShellStore: ReturnType<typeof useUiShellStore>
}

export function useWorkspaceTabs(options: WorkspaceTabsOptions) {
  const { affixTabs, resolveCurrentTab, resolveTabPath, route, router, tabs, uiShellStore } = options
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

  function onTabChange(targetKey: string) {
    const targetTab = tabs.value.find((item) => item.path === targetKey)
    void router.push(targetTab?.fullPath ?? targetKey)
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
      void router.push(fallback.fullPath)
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
    currentTab,
    currentTabPath,
    getTabCloseActionItems,
    handleTabAction,
    onTabChange,
    onTabEdit,
    tabActionItems,
  }
}
