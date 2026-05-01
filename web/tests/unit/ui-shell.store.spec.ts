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

  it('migrates remembered governance tabs to the permission policy route', () => {
    window.localStorage.setItem('rayleabot.ui-shell', JSON.stringify({
      version: 2,
      preferences: { rememberTabs: true },
      tabs: [
        {
          fullPath: '/governance',
          icon: 'governance',
          keepAlive: true,
          name: 'governance',
          path: '/governance',
          title: '权限策略',
        },
      ],
    }))

    setActivePinia(createPinia())
    const store = useUiShellStore()

    expect(store.tabs).toEqual([
      expect.objectContaining({
        fullPath: '/permission-policy',
        icon: 'permission-policy',
        keepAlive: true,
        name: 'permission-policy',
        path: '/permission-policy',
        title: '权限策略',
      }),
    ])
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

  it('closes all non-affix tabs and keeps the persisted affix tabs', () => {
    const store = useUiShellStore()

    store.upsertTab({
      affix: true,
      fullPath: '/',
      icon: 'dashboard',
      name: 'status',
      path: '/',
      title: '系统状态',
    })
    store.upsertTab({
      fullPath: '/commands',
      icon: 'commands',
      keepAlive: true,
      name: 'commands',
      path: '/commands',
      title: '指令中心',
    })
    store.upsertTab({
      fullPath: '/logs',
      icon: 'logs',
      keepAlive: true,
      name: 'logs',
      path: '/logs',
      title: '实时日志',
    })

    store.closeAllTabs()

    expect(store.tabs).toEqual([
      expect.objectContaining({
        affix: true,
        path: '/',
        title: '系统状态',
      }),
    ])

    const persisted = JSON.parse(window.localStorage.getItem('rayleabot.ui-shell') ?? '{}')
    expect(persisted.tabs).toEqual([
      expect.objectContaining({
        affix: true,
        path: '/',
        title: '系统状态',
      }),
    ])
  })
})
