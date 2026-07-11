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
      colorBgLayout: '#F3F6F7',
      colorBgContainer: '#FAF9F5',
      colorPrimary: '#0B6B8F',
      colorText: '#1F272C',
    })
    expect(variables['--auth-border']).toBe('#D8E0E4')
    expect(variables['--auth-canvas-focus']).toBe('rgb(102 204 255 / 14%)')
    expect(variables['--auth-panel-shadow']).toContain('0 24px 64px')
    expect(resolveAuthParticlePalette('light')).toEqual({
      line: 'rgb(11 107 143 / 17%)',
      particle: 'rgb(11 107 143 / 66%)',
    })
  })

  it('uses the accessible signature color and dark action text in dark mode', () => {
    const theme = resolveAuthThemeConfig('dark')
    const variables = resolveAuthCssVariables('dark')

    expect(theme.token).toMatchObject({
      colorBgLayout: '#11181C',
      colorBgContainer: '#182126',
      colorPrimary: '#66CCFF',
      colorTextLightSolid: '#11181C',
    })
    expect(variables['--auth-text-muted']).toBe('#A7B4BA')
    expect(variables['--auth-control']).toBe('#141D21')
    expect(variables['--auth-panel-highlight']).toBe('rgb(233 240 242 / 7%)')
    expect(variables['--auth-panel-shadow']).toContain('0 28px 72px')
    expect(resolveAuthParticlePalette('dark')).toEqual({
      line: 'rgb(102 204 255 / 22%)',
      particle: 'rgb(138 218 255 / 78%)',
    })
  })
})
