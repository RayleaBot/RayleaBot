import { describe, expect, it } from 'vitest'

import { buildRenderTemplatePreviewSample, parseRenderTemplatePreviewData } from '@/lib/render-template-editor'

describe('render template editor helpers', () => {
  it('locates a JSON error by line and column', () => {
    // A trailing comma on the second line leaves the third line where the parser stops.
    expect(parseRenderTemplatePreviewData('{\n  "a": 1,\n}').issue?.message).toContain('第 3 行第 1 列')
    expect(parseRenderTemplatePreviewData('[1]').issue?.message).toBe('预览输入需要是 JSON 对象。')
    expect(parseRenderTemplatePreviewData('  ').issue).toBeNull()
  })

  it('uses schema examples as preview data', () => {
    const sample = buildRenderTemplatePreviewSample({
      type: 'object',
      required: ['title'],
      properties: {
        title: { type: 'string' },
      },
      examples: [
        {
          title: '本周发言榜',
          items: [
            {
              nickname: 'Silver',
              value: 128,
            },
          ],
        },
      ],
    })

    expect(sample).toEqual({
      title: '本周发言榜',
      items: [
        {
          nickname: 'Silver',
          value: 128,
        },
      ],
    })
  })

  it('builds required object and array fields when no example is provided', () => {
    const sample = buildRenderTemplatePreviewSample({
      type: 'object',
      required: ['title', 'items'],
      properties: {
        title: { type: 'string' },
        items: {
          type: 'array',
          items: {
            type: 'object',
            required: ['nickname', 'value'],
            properties: {
              nickname: { type: 'string' },
              value: { type: ['string', 'number'] },
            },
          },
        },
        subtitle: { type: 'string' },
      },
    })

    const items = sample.items as Array<Record<string, unknown>>

    expect(sample).toMatchObject({
      items: [
        {
          value: 1,
        },
      ],
    })
    expect(typeof sample.title).toBe('string')
    expect(typeof items[0]?.nickname).toBe('string')
  })
})
