import { describe, expect, it } from 'vitest'

import {
  resolveAuthCssVariables,
  resolveAuthParticlePalette,
  resolveAuthThemeConfig,
} from '@/preferences/auth'

describe('auth theme', () => {
  it('maps the light authentication surface to the project semantic colors', () => {
    const theme = resolveAuthThemeConfig('light')
    const variables = resolveAuthCssVariables('light')

    expect(theme.token).toMatchObject({
      colorBgLayout: '#F6F3F5',
      colorBgContainer: '#FFFBFD',
      colorLink: '#8A285D',
      colorPrimary: '#8A285D',
      colorText: '#211820',
      colorTextLightSolid: '#FFFFFF',
    })
    expect(variables['--auth-border']).toBe('#DDD4D9')
    expect(variables['--auth-brand-fill']).toBe('#8A285D')
    expect(variables['--auth-brand-foreground']).toBe('#8A285D')
    expect(variables['--auth-canvas-focus']).toBe('#BF4F8724')
    expect(variables['--auth-panel-shadow']).toContain('0 2px 8px')
    expect(resolveAuthParticlePalette('light')).toEqual({
      line: '#8A285D2B',
      particle: '#8A285DA8',
    })
  })

  it('uses the accessible plum action and dark action text in dark mode', () => {
    const theme = resolveAuthThemeConfig('dark')
    const variables = resolveAuthCssVariables('dark')

    expect(theme.token).toMatchObject({
      colorBgLayout: '#151114',
      colorBgContainer: '#1D181C',
      colorPrimary: '#D57BA6',
      colorTextLightSolid: '#27101E',
    })
    expect(variables['--auth-text-muted']).toBe('#BAADB4')
    expect(variables['--auth-control']).toBe('#211B20')
    expect(variables['--auth-panel-highlight']).toBe('#282127')
    expect(variables['--auth-panel-shadow']).toContain('0 2px 10px')
    expect(resolveAuthParticlePalette('dark')).toEqual({
      line: '#F0A4C938',
      particle: '#F0A4C9C7',
    })
  })
})
