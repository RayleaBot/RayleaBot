import { describe, expect, it } from 'vitest'

import {
  defaultLayoutPreferences,
  normalizeLayoutPreferences,
} from '@/preferences/app'

describe('app preferences', () => {
  it('uses the system theme by default and preserves an explicit theme', () => {
    expect(defaultLayoutPreferences.themeMode).toBe('system')
    expect(normalizeLayoutPreferences({ themeMode: 'light' })).toEqual({
      ...defaultLayoutPreferences,
      themeMode: 'light',
    })
  })
})
