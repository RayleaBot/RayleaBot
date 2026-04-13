import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import { useUiShellStore } from '@/stores/ui-shell'

describe('ui-shell store', () => {
  beforeEach(() => {
    window.localStorage.clear()
    setActivePinia(createPinia())
  })

  it('uses the richer shell preference defaults', () => {
    const store = useUiShellStore()

    expect(store.preferences.themeMode).toBe('light')
    expect(store.preferences.pageTransition).toBe('fade-slide')
    expect(store.preferences.pageLoading).toBe(true)
    expect(store.preferences.rememberTabs).toBe(true)
  })

  it('migrates legacy local storage into the current preference shape', () => {
    window.localStorage.setItem('rayleabot.ui-shell', JSON.stringify({
      siderCollapsed: true,
      themeMode: 'dark',
    }))

    setActivePinia(createPinia())
    const store = useUiShellStore()

    expect(store.siderCollapsed).toBe(true)
    expect(store.preferences.themeMode).toBe('dark')
    expect(store.preferences.chromeTabbar).toBe(true)
  })

  it('stores tabs in local storage only when rememberTabs is enabled', () => {
    const store = useUiShellStore()

    store.upsertTab({
      fullPath: '/plugins',
      icon: 'appstore',
      keepAlive: true,
      name: 'plugins',
      path: '/plugins',
      title: '插件',
    })

    let persisted = JSON.parse(window.localStorage.getItem('rayleabot.ui-shell') ?? '{}')
    expect(persisted.tabs).toHaveLength(1)
    expect(persisted.tabs[0].icon).toBe('appstore')

    store.patchPreferences({ rememberTabs: false })
    store.upsertTab({
      fullPath: '/commands',
      keepAlive: true,
      name: 'commands',
      path: '/commands',
      title: '指令中心',
    })

    persisted = JSON.parse(window.localStorage.getItem('rayleabot.ui-shell') ?? '{}')
    expect(persisted.tabs).toBeUndefined()
  })
})
