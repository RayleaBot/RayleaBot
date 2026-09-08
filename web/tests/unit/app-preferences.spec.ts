import { describe, expect, it } from 'vitest'

import {
  defaultLayoutPreferences,
  normalizeLayoutPreferences,
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
})
