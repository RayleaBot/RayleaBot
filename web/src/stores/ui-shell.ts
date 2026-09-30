import { computed, onScopeDispose, ref } from 'vue'
import { defineStore } from 'pinia'

import {
  defaultLayoutPreferences,
  type LayoutPreferences,
  type ResolvedThemeMode,
  type ThemeMode,
  normalizeLayoutPreferences,
} from '@/preferences/app'

interface PersistedShellState {
  preferences?: Partial<LayoutPreferences>
  siderCollapsed?: boolean
  version: 3
}

const storageKey = 'rayleabot.ui-shell'
const systemThemeQuery = '(prefers-color-scheme: dark)'

function readSystemTheme(): ResolvedThemeMode {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return 'light'
  }

  return window.matchMedia(systemThemeQuery).matches ? 'dark' : 'light'
}

function readPersistedState(): PersistedShellState {
  if (typeof window === 'undefined') {
    return {
      version: 3,
    }
  }

  try {
    const raw = window.localStorage.getItem(storageKey)
    return normalizePersistedState(raw ? JSON.parse(raw) : null)
  } catch {
    return {
      version: 3,
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
      version: 3,
    }
  }

  if (!('version' in value) || (value.version !== 2 && value.version !== 3)) {
    return {
      version: 3,
    }
  }

  const nextValue = value as Partial<PersistedShellState>
  return {
    version: 3,
    preferences: normalizeLayoutPreferences(nextValue.preferences),
    siderCollapsed: Boolean(nextValue.siderCollapsed),
  }
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
  const systemTheme = ref<ResolvedThemeMode>(readSystemTheme())
  const themeMode = computed(() => preferences.value.themeMode)
  const resolvedThemeMode = computed<ResolvedThemeMode>(() => (
    themeMode.value === 'system' ? systemTheme.value : themeMode.value
  ))

  let systemThemeMediaQuery: MediaQueryList | null = null
  const handleSystemThemeChange = (event: MediaQueryListEvent) => {
    systemTheme.value = event.matches ? 'dark' : 'light'
  }

  if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
    systemThemeMediaQuery = window.matchMedia(systemThemeQuery)
    systemThemeMediaQuery.addEventListener?.('change', handleSystemThemeChange)
  }

  onScopeDispose(() => {
    systemThemeMediaQuery?.removeEventListener?.('change', handleSystemThemeChange)
  })

  function persist() {
    writePersistedState({
      version: 3,
      preferences: preferences.value,
      siderCollapsed: siderCollapsed.value,
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

  function resetPreferences() {
    preferences.value = { ...defaultLayoutPreferences }
    persist()
  }

  return {
    mobileMenuOpen,
    preferences,
    resolvedThemeMode,
    patchPreferences,
    resetPreferences,
    routeLoading,
    searchOpen,
    siderCollapsed,
    settingsOpen,
    themeMode,
    closeSearch,
    closeSettings,
    setMobileMenuOpen,
    setRouteLoading,
    setThemeMode,
    toggleSider,
    openSearch,
    openSettings,
  }
})
