import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useRenderTemplatesStore } from '@/stores/render-templates'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function templateDetail(updatedAt = '2026-04-18T10:30:00Z') {
  return {
    id: 'help.menu',
    version: '1',
    width: 960,
    height: 640,
    has_input_schema: true,
    updated_at: updatedAt,
    input_schema_json: {
      type: 'object',
      properties: {
        title: { type: 'string', description: '主标题' },
      },
      required: ['title'],
    },
  } as const
}

describe('render templates store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('sorts template summaries by updated_at descending', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(jsonResponse({
        items: [
          {
            id: 'status.panel',
            version: '1',
            width: 960,
            height: 540,
            has_input_schema: true,
            updated_at: '2026-04-18T09:45:00Z',
          },
          {
            id: 'help.menu',
            version: '1',
            width: 960,
            height: 640,
            has_input_schema: true,
            updated_at: '2026-04-18T10:30:00Z',
          },
        ],
      })),
    )

    const store = useRenderTemplatesStore()
    await store.fetchTemplates()

    expect(store.items.map((item) => item.id)).toEqual(['help.menu', 'status.panel'])
  })

  it('loads one preview workspace detail and upserts the summary list', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(jsonResponse({ template: templateDetail() })),
    )

    const store = useRenderTemplatesStore()
    store.items = [
      {
        id: 'status.panel',
        version: '1',
        width: 960,
        height: 540,
        has_input_schema: true,
        updated_at: '2026-04-18T09:45:00Z',
      },
    ]

    await store.fetchTemplateWorkspace('help.menu')

    expect(store.detailById['help.menu']?.input_schema_json).toEqual(templateDetail().input_schema_json)
    expect(store.items.map((item) => item.id)).toEqual(['help.menu', 'status.panel'])
  })
})
