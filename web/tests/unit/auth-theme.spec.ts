import { describe, expect, it } from 'vitest'

import {
  resolveAuthCssVariables,
} from '@/preferences/auth'
import { webThemes } from '@/preferences/theme-tokens'

describe('auth theme', () => {
  it('maps the light authentication surface to the project semantic colors', () => {
    const tokens = webThemes.light
    const variables = resolveAuthCssVariables('light')

    expect(variables['--auth-border']).toBe(tokens.border)
    expect(variables['--auth-brand-fill']).toBe(tokens.brandFill)
    expect(variables['--auth-brand-foreground']).toBe(tokens.brandForeground)
    expect(variables['--auth-canvas-focus']).toBe(tokens.authCanvasFocus)
    expect(variables['--auth-panel-shadow']).toBe(tokens.shadowFloating)
  })

  it('uses the accessible celadon action and dark action text in dark mode', () => {
    const tokens = webThemes.dark
    const variables = resolveAuthCssVariables('dark')

    expect(variables['--auth-on-brand']).toBe(tokens.onBrand)
    expect(variables['--auth-focus']).toBe(tokens.focus)
    expect(variables['--auth-text-muted']).toBe(tokens.textMuted)
    expect(variables['--auth-control']).toBe(tokens.authControl)
    expect(variables['--auth-panel-highlight']).toBe(tokens.surfaceRaised)
    expect(variables['--auth-panel-shadow']).toBe(tokens.shadowFloating)
  })
})
