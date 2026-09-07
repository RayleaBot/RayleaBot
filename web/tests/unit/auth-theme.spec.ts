import { describe, expect, it } from 'vitest'

import {
  resolveAuthCssVariables,
  resolveAuthThemeConfig,
} from '@/preferences/auth'
import { webThemes } from '@/preferences/theme-tokens'

describe('auth theme', () => {
  it('maps the light authentication surface to the project semantic colors', () => {
    const tokens = webThemes.light
    const theme = resolveAuthThemeConfig('light')
    const variables = resolveAuthCssVariables('light')

    expect(theme.token).toMatchObject({
      colorBgLayout: tokens.canvas,
      colorBgContainer: tokens.surface,
      colorLink: tokens.brandForeground,
      colorPrimary: tokens.brandFill,
      colorText: tokens.text,
      colorTextLightSolid: tokens.onBrand,
      controlHeight: 50,
    })
    expect(theme.components?.Button).toMatchObject({ controlHeight: 50 })
    expect(theme.components?.Input).toMatchObject({ controlOutline: 'transparent', colorPrimary: tokens.focus, controlHeight: 50 })
    expect(variables['--auth-border']).toBe(tokens.border)
    expect(variables['--auth-brand-fill']).toBe(tokens.brandFill)
    expect(variables['--auth-brand-foreground']).toBe(tokens.brandForeground)
    expect(variables['--auth-canvas-focus']).toBe(tokens.authCanvasFocus)
    expect(variables['--auth-panel-shadow']).toBe(tokens.shadowFloating)
  })

  it('uses the accessible celadon action and dark action text in dark mode', () => {
    const tokens = webThemes.dark
    const theme = resolveAuthThemeConfig('dark')
    const variables = resolveAuthCssVariables('dark')

    expect(theme.token).toMatchObject({
      colorBgLayout: tokens.canvas,
      colorBgContainer: tokens.surface,
      colorPrimary: tokens.brandFill,
      colorTextLightSolid: tokens.onBrand,
    })
    expect(variables['--auth-text-muted']).toBe(tokens.textMuted)
    expect(variables['--auth-control']).toBe(tokens.authControl)
    expect(variables['--auth-panel-highlight']).toBe(tokens.surfaceRaised)
    expect(theme.components?.Input).toMatchObject({ controlOutline: 'transparent', colorPrimary: tokens.focus, controlHeight: 50 })
    expect(variables['--auth-panel-shadow']).toBe(tokens.shadowFloating)
  })
})
