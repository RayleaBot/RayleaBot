import { describe, expect, it } from 'vitest'
import { normalizeSettings } from '../src/model'

describe('config panel model', () => {
  it('normalizes unsupported units', () => {
    expect(normalizeSettings({ default_city: '上海', unit: 'kelvin' })).toEqual({ default_city: '上海', unit: 'celsius' })
  })
})
