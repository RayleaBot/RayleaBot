import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useUiShellStore } from '@/stores/ui-shell'
import { createPluginCenterTab } from '@/access/plugin-center'

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

  it('merges stored plugin tabs at their earliest position and retains independent details', () => {
    const tab = (name: string, path: string) => ({ name, path, fullPath: path, title: name, keepAlive: true })
    window.localStorage.setItem('rayleabot.ui-shell', JSON.stringify({
      version: 3,
      preferences: { rememberTabs: true },
      tabs: [tab('logs', '/logs'), tab('commands', '/commands'), tab('plugin-detail', '/plugins/weather'), tab('plugin-settings', '/plugins/settings'), tab('menu-center', '/menu-center')],
    }))
    const store = useUiShellStore()
    expect(store.tabs.map(item => item.path)).toEqual(['/logs', '/plugins', '/plugins/weather'])
    expect(store.tabs[1]).toMatchObject({ name: 'plugin-center', fullPath: '/plugins' })
    expect(store.effectiveCachedViewNames).toEqual(expect.arrayContaining(['logs', 'plugins', 'plugin-settings', 'commands', 'menu-center', 'plugin-store']))
    store.removeTab('/plugins')
    expect(store.tabs.map(item => item.path)).toEqual(['/logs', '/plugins/weather'])
    expect(store.effectiveCachedViewNames).not.toContain('plugin-settings')
  })

  it('restores the merged full URL and rejects unrelated destinations', () => {
    const saved = createPluginCenterTab('/commands?plugin_id=weather#list')
    const seed = (fullPath: string) => window.localStorage.setItem('rayleabot.ui-shell', JSON.stringify({
      version: 3, preferences: { rememberTabs: true }, tabs: [{ ...saved, fullPath }],
    }))
    seed(saved.fullPath)
    expect(useUiShellStore().tabs[0]?.fullPath).toBe(saved.fullPath)
    setActivePinia(createPinia())
    seed('/plugins/weather')
    expect(useUiShellStore().tabs[0]?.fullPath).toBe('/plugins')
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

  it('restores persisted tabs and closes them to empty storage', () => {
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

    store.closeAllTabs()

    expect(store.tabs).toEqual([])
    const persisted = JSON.parse(window.localStorage.getItem('rayleabot.ui-shell') ?? '{}')
    expect(persisted.tabs).toEqual([])
  })
})
