import { describe, expect, it } from 'vitest'

import {
  resolveAuthCssVariables,
  resolveAuthThemeConfig,
} from '@/preferences/auth'

describe('auth theme', () => {
  it('maps the light authentication surface to the project semantic colors', () => {
    const theme = resolveAuthThemeConfig('light')
    const variables = resolveAuthCssVariables('light')

    expect(theme.token).toMatchObject({
      colorBgLayout: '#FAFAFA',
      colorBgContainer: '#FFFFFF',
      colorLink: '#476C5E',
      colorPrimary: '#476C5E',
      colorText: '#252525',
      colorTextLightSolid: '#FFFFFF',
    })
    expect(variables['--auth-border']).toBe('#E3E3E3')
    expect(variables['--auth-brand-fill']).toBe('#476C5E')
    expect(variables['--auth-brand-foreground']).toBe('#476C5E')
    expect(variables['--auth-canvas-focus']).toBe('#476C5E1F')
    expect(variables['--auth-panel-shadow']).toContain('0 2px 8px')
  })

  it('uses the accessible celadon action and dark action text in dark mode', () => {
    const theme = resolveAuthThemeConfig('dark')
    const variables = resolveAuthCssVariables('dark')

    expect(theme.token).toMatchObject({
      colorBgLayout: '#161616',
      colorBgContainer: '#1E1E1E',
      colorPrimary: '#A3C5B3',
      colorTextLightSolid: '#18281F',
    })
    expect(variables['--auth-text-muted']).toBe('#ADADAD')
    expect(variables['--auth-control']).toBe('#252525')
    expect(variables['--auth-panel-highlight']).toBe('#292929')
    expect(variables['--auth-panel-shadow']).toContain('0 2px 10px')
  })
})
