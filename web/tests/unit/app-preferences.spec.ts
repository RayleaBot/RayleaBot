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
      colorBgBase: '#F6F3F5',
      colorLink: '#8A285D',
      colorPrimary: '#8A285D',
      colorPrimaryHover: '#6C204B',
      colorPrimaryActive: '#3F1830',
      colorText: '#211820',
      colorTextLightSolid: '#FFFFFF',
      controlOutline: '#9F356C',
    }))
    expect(resolveThemeConfig('dark', 'compact').token).toEqual(expect.objectContaining({
      colorBgBase: '#151114',
      colorPrimary: '#D57BA6',
      colorPrimaryHover: '#E8AAC7',
      colorPrimaryActive: '#BF4F87',
      colorText: '#F4EDF1',
      colorTextLightSolid: '#27101E',
      controlOutline: '#F0A4C9',
    }))
  })

  it('keeps the CSS brand roles aligned with the Ant theme', () => {
    expect(designTokens).toContain('--brand-fill: #8A285D;')
    expect(designTokens).toContain('--brand-fill-hover: #6C204B;')
    expect(designTokens).toContain('--brand-foreground: #8A285D;')
    expect(designTokens).toContain('--on-brand: #FFFFFF;')
    expect(designTokens).toContain('--sider-bg: #3F1830;')
  })
})
