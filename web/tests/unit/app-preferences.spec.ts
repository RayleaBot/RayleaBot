import { describe, expect, it } from 'vitest'

import {
  defaultLayoutPreferences,
  normalizeLayoutPreferences,
  resolveThemeConfig,
} from '@/preferences/app'
import designTokens from '@/styles/_theme-tokens.generated.scss?raw'

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
      colorBgBase: '#FAFAFA',
      colorLink: '#476C5E',
      colorPrimary: '#476C5E',
      colorPrimaryHover: '#365749',
      colorPrimaryActive: '#294438',
      colorText: '#252525',
      colorTextLightSolid: '#FFFFFF',
      controlOutline: '#555555',
    }))
    expect(resolveThemeConfig('dark', 'compact').token).toEqual(expect.objectContaining({
      colorBgBase: '#161616',
      colorPrimary: '#A3C5B3',
      colorPrimaryHover: '#BBD0C1',
      colorPrimaryActive: '#80A48F',
      colorText: '#EEEEEE',
      colorTextLightSolid: '#18281F',
      controlOutline: '#C0C0C0',
    }))
  })

  it('keeps the CSS brand roles aligned with the Ant theme', () => {
    expect(designTokens).toContain('--brand-fill: #476C5E;')
    expect(designTokens).toContain('--brand-fill-hover: #365749;')
    expect(designTokens).toContain('--brand-foreground: #476C5E;')
    expect(designTokens).toContain('--on-brand: #FFFFFF;')
    expect(designTokens).toContain('--sider-bg: #F7F7F7;')
  })
})
