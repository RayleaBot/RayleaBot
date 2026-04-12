import { describe, expect, it } from 'vitest'

import { defaultLayoutPreferences } from '@/preferences/app'

describe('defaultLayoutPreferences', () => {
  it('uses light theme by default', () => {
    expect(defaultLayoutPreferences.themeMode).toBe('light')
  })
})
