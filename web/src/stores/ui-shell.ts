import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import {
  defaultLayoutPreferences,
  type LayoutPreferences,
  type ThemeMode,
} from '@/preferences/app'

export interface ShellTabItem {
  affix?: boolean
  fullPath: string
  name: string
  path: string
  title: string
}

interface PersistedShellState {
  siderCollapsed?: boolean
  themeMode?: ThemeMode
}

const storageKey = 'rayleabot.ui-shell'

function readPersistedState(): PersistedShellState {
  if (typeof window === 'undefined') {
    return {}
  }

  try {
    const raw = window.localStorage.getItem(storageKey)
    return raw ? (JSON.parse(raw) as PersistedShellState) : {}
  } catch {
    return {}
  }
}

function writePersistedState(state: PersistedShellState) {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.setItem(storageKey, JSON.stringify(state))
}

export const useUiShellStore = defineStore('ui-shell', () => {
  const persistedState = readPersistedState()
  const siderCollapsed = ref(Boolean(persistedState.siderCollapsed))
  const mobileMenuOpen = ref(false)
  const themeMode = ref<ThemeMode>(persistedState.themeMode ?? defaultLayoutPreferences.themeMode)
  const tabs = ref<ShellTabItem[]>([])

  const preferences = computed<LayoutPreferences>(() => ({
    ...defaultLayoutPreferences,
    themeMode: themeMode.value,
  }))

  function persist() {
    writePersistedState({
      siderCollapsed: siderCollapsed.value,
      themeMode: themeMode.value,
    })
  }

  function toggleSider() {
    siderCollapsed.value = !siderCollapsed.value
    persist()
  }

  function setMobileMenuOpen(nextValue: boolean) {
    mobileMenuOpen.value = nextValue
  }

  function setThemeMode(nextValue: ThemeMode) {
    themeMode.value = nextValue
    persist()
  }

  function toggleThemeMode() {
    setThemeMode(themeMode.value === 'dark' ? 'light' : 'dark')
  }

  function syncTabs(affixTabs: ShellTabItem[]) {
    const currentTabs = tabs.value.filter((item) => !item.affix)
    const nextTabs = [...affixTabs]

    for (const item of currentTabs) {
      if (!nextTabs.some((candidate) => candidate.path === item.path)) {
        nextTabs.push(item)
      }
    }

    tabs.value = nextTabs
  }

  function upsertTab(tab: ShellTabItem) {
    const existingIndex = tabs.value.findIndex((item) => item.path === tab.path)
    if (existingIndex >= 0) {
      tabs.value.splice(existingIndex, 1, { ...tabs.value[existingIndex], ...tab })
      return
    }

    tabs.value.push(tab)
  }

  function removeTab(path: string) {
    tabs.value = tabs.value.filter((item) => item.path !== path || item.affix)
  }

  return {
    mobileMenuOpen,
    preferences,
    siderCollapsed,
    tabs,
    themeMode,
    removeTab,
    setMobileMenuOpen,
    setThemeMode,
    syncTabs,
    toggleSider,
    toggleThemeMode,
    upsertTab,
  }
})
