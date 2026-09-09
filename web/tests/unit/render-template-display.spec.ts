import { describe, expect, it } from 'vitest'
import { getRenderTemplateTypeLabel } from '@/lib/render-template-display'

describe('template schema labels', () => {
  it('translates combined schema types', () => {
    expect(getRenderTemplateTypeLabel('string | null')).toBe('文本 / 空值')
  })
})
