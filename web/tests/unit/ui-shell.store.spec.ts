import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useUiShellStore } from '@/stores/ui-shell'

describe('ui-shell store', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    window.localStorage.clear()
    setActivePinia(createPinia())
  })

  it('restores the current shell preference shape', () => {
    window.localStorage.setItem('rayleabot.ui-shell', JSON.stringify({
      version: 2,
      preferences: { rememberTabs: true, themeMode: 'dark' },
      siderCollapsed: true,
      tabs: [
        {
          fullPath: '/permission-policy',
          icon: 'permission-policy',
          keepAlive: true,
          name: 'permission-policy',
          path: '/permission-policy',
          title: '权限策略',
        },
      ],
    }))

    setActivePinia(createPinia())
    const store = useUiShellStore()

    expect(store.siderCollapsed).toBe(true)
    expect(store.preferences.themeMode).toBe('dark')
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

  it('follows system color changes only while system mode is selected', () => {
    let listener: ((event: MediaQueryListEvent) => void) | undefined
    const removeEventListener = vi.fn()
    vi.spyOn(window, 'matchMedia').mockReturnValue({
      matches: true,
      addEventListener: (_type: string, nextListener: (event: MediaQueryListEvent) => void) => {
        listener = nextListener
      },
      removeEventListener,
    } as unknown as MediaQueryList)

    const store = useUiShellStore()
    expect(store.preferences.themeMode).toBe('system')
    expect(store.resolvedThemeMode).toBe('dark')

    listener?.({ matches: false } as MediaQueryListEvent)
    expect(store.resolvedThemeMode).toBe('light')

    store.setThemeMode('dark')
    listener?.({ matches: false } as MediaQueryListEvent)
    expect(store.resolvedThemeMode).toBe('dark')

    store.$dispose()
    expect(removeEventListener).toHaveBeenCalledOnce()
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

  it('resets restored tabs and keeps only affix tabs in storage', () => {
    const store = useUiShellStore()

    store.syncTabs([
      {
        affix: true,
        fullPath: '/',
        icon: 'dashboard',
        name: 'status',
        path: '/',
        title: '系统状态',
      },
    ])
    store.upsertTab({
      fullPath: '/plugins',
      icon: 'appstore',
      keepAlive: true,
      name: 'plugins',
      path: '/plugins',
      title: '插件列表',
    })
    store.upsertTab({
      fullPath: '/commands',
      icon: 'commands',
      keepAlive: true,
      name: 'commands',
      path: '/commands',
      title: '指令中心',
    })

    store.resetRestoredTabs()

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

  it('resets restored tabs to empty storage before affix tabs are synced', () => {
    window.localStorage.setItem('rayleabot.ui-shell', JSON.stringify({
      version: 2,
      preferences: { rememberTabs: true },
      tabs: [
        {
          fullPath: '/commands',
          keepAlive: true,
          name: 'commands',
          path: '/commands',
          title: '指令中心',
        },
      ],
    }))

    setActivePinia(createPinia())
    const store = useUiShellStore()

    store.resetRestoredTabs()

    expect(store.tabs).toEqual([])
    const persisted = JSON.parse(window.localStorage.getItem('rayleabot.ui-shell') ?? '{}')
    expect(persisted.tabs).toEqual([])
  })
})
