import { describe, expect, it } from 'vitest'

import { normalizeSettings, validateSettings } from '../src/model'

describe('fortune settings model', () => {
  it('normalizes duplicate triggers and validates an empty fortune pool', () => {
    const settings = normalizeSettings({ trigger_commands: ['运势', '运势'], fortunes: [] })
    expect(settings.trigger_commands).toEqual(['运势'])
    expect(validateSettings(settings)).toContain('至少需要一条运势。')
  })
})
