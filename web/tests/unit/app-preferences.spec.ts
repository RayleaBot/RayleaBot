import { describe, expect, it } from 'vitest'

import {
  defaultLayoutPreferences,
  normalizeLayoutPreferences,
  resolveThemeConfig,
} from '@/preferences/app'
import { webThemes } from '@/preferences/theme-tokens'

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
    const light = webThemes.light
    expect(resolveThemeConfig('light', 'default').token).toEqual(expect.objectContaining({
      colorBgBase: light.canvas,
      colorLink: light.brandForeground,
      colorPrimary: light.brandFill,
      colorPrimaryHover: light.brandFillHover,
      colorPrimaryActive: light.brandFillPressed,
      colorText: light.text,
      colorTextLightSolid: light.onBrand,
      controlOutline: light.focus,
    }))
    const dark = webThemes.dark
    expect(resolveThemeConfig('dark', 'compact').token).toEqual(expect.objectContaining({
      colorBgBase: dark.canvas,
      colorPrimary: dark.brandFill,
      colorPrimaryHover: dark.brandFillHover,
      colorPrimaryActive: dark.brandFillPressed,
      colorText: dark.text,
      colorTextLightSolid: dark.onBrand,
      controlOutline: dark.focus,
    }))
  })
})
