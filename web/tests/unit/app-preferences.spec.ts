import { describe, expect, it } from 'vitest'

import {
  defaultLayoutPreferences,
  normalizeLayoutPreferences,
  resolveThemeConfig,
} from '@/preferences/app'

describe('app preferences', () => {
  it('defaults to the system theme and drops retired visual overrides', () => {
    const preferences = normalizeLayoutPreferences({
      themeMode: 'light',
      primaryColor: '#ff00ff',
      borderRadius: 24,
      fontScale: 'sm',
      fixedHeader: false,
      breadcrumb: false,
      pageLoading: false,
    } as never)

    expect(defaultLayoutPreferences.themeMode).toBe('system')
    expect(preferences).toEqual(expect.objectContaining({ themeMode: 'light' }))
    expect(preferences).not.toHaveProperty('primaryColor')
    expect(preferences).not.toHaveProperty('borderRadius')
    expect(preferences).not.toHaveProperty('fontScale')
    expect(preferences).not.toHaveProperty('fixedHeader')
  })

  it('projects the design palette into equivalent light and dark Ant tokens', () => {
    expect(resolveThemeConfig('light', 'default').token).toEqual(expect.objectContaining({
      colorBgBase: '#F3F6F7',
      colorPrimary: '#0B6B8F',
      colorText: '#1F272C',
    }))
    expect(resolveThemeConfig('dark', 'compact').token).toEqual(expect.objectContaining({
      colorBgBase: '#11181C',
      colorPrimary: '#66CCFF',
      colorText: '#E9F0F2',
    }))
  })
})
