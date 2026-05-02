import { describe, expect, it } from 'vitest'

import { buildRenderTemplatePreviewSample } from '@/lib/render-template-editor'

describe('render template editor helpers', () => {
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

    expect(sample).toEqual({
      title: '示例标题',
      items: [
        {
          nickname: '示例成员',
          value: 1,
        },
      ],
    })
  })
})
