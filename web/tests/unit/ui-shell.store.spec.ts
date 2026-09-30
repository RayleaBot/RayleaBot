import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useUiShellStore } from '@/stores/ui-shell'

describe('ui-shell store', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    window.localStorage.clear()
    setActivePinia(createPinia())
  })

  it('restores the current shell preference shape and drops tabs saved by earlier versions', () => {
    window.localStorage.setItem('rayleabot.ui-shell', JSON.stringify({
      version: 2,
      preferences: { rememberTabs: true, themeMode: 'dark' },
      siderCollapsed: true,
      tabs: [{ fullPath: '/permission-policy', name: 'permission-policy', path: '/permission-policy', title: '权限策略' }],
    }))

    setActivePinia(createPinia())
    const store = useUiShellStore()

    expect(store.siderCollapsed).toBe(true)
    expect(store.preferences.themeMode).toBe('dark')
    expect(store.preferences).not.toHaveProperty('rememberTabs')

    store.toggleSider()
    expect(JSON.parse(window.localStorage.getItem('rayleabot.ui-shell') ?? '{}')).toEqual({
      version: 3,
      preferences: store.preferences,
      siderCollapsed: false,
    })
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
})
