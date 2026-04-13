import { describe, expect, it } from 'vitest'

import { defaultLayoutPreferences, normalizeLayoutPreferences } from '@/preferences/app'

describe('defaultLayoutPreferences', () => {
  it('uses light theme by default', () => {
    expect(defaultLayoutPreferences.themeMode).toBe('light')
    expect(defaultLayoutPreferences.pageTransition).toBe('fade-slide')
    expect(defaultLayoutPreferences.rememberTabs).toBe(true)
  })

  it('normalizes partial preferences onto the shared defaults', () => {
    const preferences = normalizeLayoutPreferences({
      themeMode: 'dark',
      pageTransition: 'fade',
      contentWidth: 'fixed',
    })

    expect(preferences.themeMode).toBe('dark')
    expect(preferences.pageTransition).toBe('fade')
    expect(preferences.contentWidth).toBe('fixed')
    expect(preferences.chromeTabbar).toBe(true)
  })
})
