import { computed, nextTick, ref } from 'vue'
import { defineStore } from 'pinia'

import {
  defaultLayoutPreferences,
  type LayoutPreferences,
  type ThemeMode,
  normalizeLayoutPreferences,
} from '@/preferences/app'

export interface ShellTabItem {
  affix?: boolean
  fullPath: string
  icon?: string
  keepAlive?: boolean
  name: string
  path: string
  title: string
}

interface LegacyPersistedShellState {
  siderCollapsed?: boolean
  themeMode?: ThemeMode
}

interface PersistedShellState {
  preferences?: Partial<LayoutPreferences>
  siderCollapsed?: boolean
  tabs?: ShellTabItem[]
  version: 2
}

const storageKey = 'rayleabot.ui-shell'

function readPersistedState(): PersistedShellState {
  if (typeof window === 'undefined') {
    return {
      version: 2,
    }
  }

  try {
    const raw = window.localStorage.getItem(storageKey)
    return normalizePersistedState(raw ? JSON.parse(raw) : null)
  } catch {
    return {
      version: 2,
    }
  }
}

function writePersistedState(state: PersistedShellState) {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.setItem(storageKey, JSON.stringify(state))
}

function normalizePersistedState(value: unknown): PersistedShellState {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return {
      version: 2,
    }
  }

  if ('version' in value && value.version === 2) {
    const nextValue = value as Partial<PersistedShellState>
    return {
      version: 2,
      preferences: normalizeLayoutPreferences(nextValue.preferences),
      siderCollapsed: Boolean(nextValue.siderCollapsed),
      tabs: normalizeTabs(nextValue.tabs),
    }
  }

  const legacyValue = value as LegacyPersistedShellState
  return {
    version: 2,
    preferences: normalizeLayoutPreferences({
      themeMode: legacyValue.themeMode ?? defaultLayoutPreferences.themeMode,
    }),
    siderCollapsed: Boolean(legacyValue.siderCollapsed),
    tabs: [],
  }
}

function normalizeTabs(value: unknown): ShellTabItem[] {
  if (!Array.isArray(value)) {
    return []
  }

  const nextTabs: ShellTabItem[] = []

  for (const item of value) {
    if (!item || typeof item !== 'object' || Array.isArray(item)) {
      continue
    }

    const tab = item as Partial<ShellTabItem>
    if (
      typeof tab.fullPath !== 'string'
      || typeof tab.name !== 'string'
      || typeof tab.path !== 'string'
      || typeof tab.title !== 'string'
    ) {
      continue
    }

    nextTabs.push(normalizeLegacyTab({
      affix: Boolean(tab.affix),
      fullPath: tab.fullPath,
      icon: typeof tab.icon === 'string' && tab.icon ? tab.icon : undefined,
      keepAlive: Boolean(tab.keepAlive),
      name: tab.name,
      path: tab.path,
      title: tab.title,
    }))
  }

  return dedupeTabs(nextTabs)
}

function normalizeLegacyTab(tab: ShellTabItem): ShellTabItem {
  if (tab.name !== 'governance' && tab.path !== '/governance' && tab.fullPath !== '/governance') {
    return tab
  }

  return {
    ...tab,
    fullPath: '/permission-policy',
    icon: 'permission-policy',
    keepAlive: true,
    name: 'permission-policy',
    path: '/permission-policy',
    title: tab.title || '权限策略',
  }
}

function dedupeTabs(items: ShellTabItem[]) {
  const nextItems = new Map<string, ShellTabItem>()
  for (const item of items) {
    nextItems.set(item.path, item)
  }
  return Array.from(nextItems.values())
}

export const useUiShellStore = defineStore('ui-shell', () => {
  const persistedState = readPersistedState()
  const preferences = ref<LayoutPreferences>(
    normalizeLayoutPreferences(persistedState.preferences ?? defaultLayoutPreferences),
  )
  const siderCollapsed = ref(Boolean(persistedState.siderCollapsed))
  const mobileMenuOpen = ref(false)
  const searchOpen = ref(false)
  const settingsOpen = ref(false)
  const routeLoading = ref(false)
  const excludedViewNames = ref<string[]>([])
  const refreshKeys = ref<Record<string, number>>({})
  const tabs = ref<ShellTabItem[]>(
    normalizeTabs(preferences.value.rememberTabs ? persistedState.tabs : []),
  )
  const themeMode = computed(() => preferences.value.themeMode)
  const cachedViewNames = computed(() => {
    const names = tabs.value
      .filter((item) => item.keepAlive)
      .map((item) => item.name)

    return Array.from(new Set(names))
  })
  const effectiveCachedViewNames = computed(() => (
    cachedViewNames.value.filter((name) => !excludedViewNames.value.includes(name))
  ))

  function persist() {
    writePersistedState({
      version: 2,
      preferences: preferences.value,
      siderCollapsed: siderCollapsed.value,
      tabs: preferences.value.rememberTabs ? tabs.value : undefined,
    })
  }

  function toggleSider() {
    siderCollapsed.value = !siderCollapsed.value
    persist()
  }

  function setMobileMenuOpen(nextValue: boolean) {
    mobileMenuOpen.value = nextValue
  }

  function patchPreferences(nextValue: Partial<LayoutPreferences>) {
    preferences.value = normalizeLayoutPreferences({
      ...preferences.value,
      ...nextValue,
    })
    persist()
  }

  function setThemeMode(nextValue: ThemeMode) {
    patchPreferences({ themeMode: nextValue })
  }

  function toggleThemeMode() {
    setThemeMode(themeMode.value === 'dark' ? 'light' : 'dark')
  }

  function syncTabs(affixTabs: ShellTabItem[]) {
    const currentTabs = tabs.value.filter((item) => !item.affix)
    const nextTabs = normalizeTabs(affixTabs).map((item) => ({
      ...item,
      affix: true,
    }))

    for (const item of currentTabs) {
      if (!nextTabs.some((candidate) => candidate.path === item.path)) {
        nextTabs.push(item)
      }
    }

    tabs.value = dedupeTabs(nextTabs)
    persist()
  }

  function upsertTab(tab: ShellTabItem) {
    const normalizedTab = normalizeTabs([tab])[0]
    if (!normalizedTab) {
      return
    }

    const existingIndex = tabs.value.findIndex((item) => item.path === tab.path)
    if (existingIndex >= 0) {
      tabs.value.splice(existingIndex, 1, { ...tabs.value[existingIndex], ...normalizedTab })
      persist()
      return
    }

    tabs.value.push(normalizedTab)
    persist()
  }

  function removeTab(path: string) {
    tabs.value = tabs.value.filter((item) => item.path !== path || item.affix)
    persist()
  }

  function removeTabsByName(name: string, options: { exceptPath?: string } = {}) {
    tabs.value = tabs.value.filter((item) => {
      if (item.affix) {
        return true
      }

      if (item.name !== name) {
        return true
      }

      return item.path === options.exceptPath
    })
    persist()
  }

  function closeOtherTabs(path: string) {
    tabs.value = tabs.value.filter((item) => item.affix || item.path === path)
    persist()
  }

  function closeTabsToLeft(path: string) {
    const targetIndex = tabs.value.findIndex((item) => item.path === path)
    if (targetIndex < 0) {
      return
    }

    tabs.value = tabs.value.filter((item, index) => item.affix || index >= targetIndex)
    persist()
  }

  function closeTabsToRight(path: string) {
    const targetIndex = tabs.value.findIndex((item) => item.path === path)
    if (targetIndex < 0) {
      return
    }

    tabs.value = tabs.value.filter((item, index) => item.affix || index <= targetIndex)
    persist()
  }

  function closeAllTabs() {
    tabs.value = tabs.value.filter((item) => item.affix)
    persist()
  }

  function resetRestoredTabs() {
    closeAllTabs()
  }

  function openSearch() {
    searchOpen.value = true
  }

  function closeSearch() {
    searchOpen.value = false
  }

  function openSettings() {
    settingsOpen.value = true
  }

  function closeSettings() {
    settingsOpen.value = false
  }

  function setRouteLoading(nextValue: boolean) {
    routeLoading.value = nextValue
  }

  async function refreshView(name: string) {
    if (cachedViewNames.value.includes(name) && !excludedViewNames.value.includes(name)) {
      excludedViewNames.value = [...excludedViewNames.value, name]
      await nextTick()
      excludedViewNames.value = excludedViewNames.value.filter((item) => item !== name)
    }

    refreshKeys.value = {
      ...refreshKeys.value,
      [name]: (refreshKeys.value[name] ?? 0) + 1,
    }
  }

  function getRefreshKey(name: string) {
    return refreshKeys.value[name] ?? 0
  }

  function resetPreferences() {
    preferences.value = { ...defaultLayoutPreferences }
    persist()
  }

  return {
    cachedViewNames,
    effectiveCachedViewNames,
    closeAllTabs,
    closeOtherTabs,
    closeTabsToLeft,
    closeTabsToRight,
    getRefreshKey,
    mobileMenuOpen,
    preferences,
    patchPreferences,
    refreshView,
    resetPreferences,
    resetRestoredTabs,
    routeLoading,
    searchOpen,
    siderCollapsed,
    settingsOpen,
    tabs,
    themeMode,
    closeSearch,
    closeSettings,
    removeTab,
    removeTabsByName,
    setMobileMenuOpen,
    setRouteLoading,
    setThemeMode,
    syncTabs,
    toggleSider,
    toggleThemeMode,
    upsertTab,
    openSearch,
    openSettings,
  }
})
