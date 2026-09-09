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
    name: '测试模板',
    version: '1',
    width: 960,
    height: 640,
    has_input_schema: true,
    updated_at: updatedAt,
    source: {
      type: 'system',
      plugin_id: null,
      local_id: null,
    },
    preview_data_json: { title: '示例' },
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
            name: '测试模板',
            version: '1',
            width: 960,
            height: 540,
            has_input_schema: true,
            updated_at: '2026-04-18T09:45:00Z',
            source: {
              type: 'system',
              plugin_id: null,
              local_id: null,
            },
          },
          {
            id: 'help.menu',
            name: '测试模板',
            version: '1',
            width: 960,
            height: 640,
            has_input_schema: true,
            updated_at: '2026-04-18T10:30:00Z',
            source: {
              type: 'system',
              plugin_id: null,
              local_id: null,
            },
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
        name: '测试模板',
        version: '1',
        width: 960,
        height: 540,
        has_input_schema: true,
        updated_at: '2026-04-18T09:45:00Z',
        source: {
          type: 'system',
          plugin_id: null,
          local_id: null,
        },
      },
    ]

    await store.fetchTemplateWorkspace('help.menu')

    expect(store.detailById['help.menu']?.input_schema_json).toEqual(templateDetail().input_schema_json)
    expect(store.items.map((item) => item.id)).toEqual(['help.menu', 'status.panel'])
  })

  it('keeps refreshed content when an old detail request completes later', async () => {
    let resolveDetail: (value: Response) => void = () => {}
    const current = templateDetail('2026-05-01T00:00:00Z')
    vi.stubGlobal('fetch', vi.fn()
      .mockResolvedValueOnce(jsonResponse({ items: [templateDetail()] }))
      .mockImplementationOnce(() => new Promise<Response>(resolve => { resolveDetail = resolve }))
      .mockResolvedValueOnce(jsonResponse({ items: [current] })))
    const store = useRenderTemplatesStore()
    await store.fetchTemplates()
    const pending = store.fetchTemplateWorkspace('help.menu')
    await store.fetchTemplates()
    resolveDetail(jsonResponse({ template: templateDetail() }))
    await pending
    expect(store.items[0]?.updated_at).toBe(current.updated_at)
    expect(store.detailById).toEqual({})
  })

  it('keeps the newest detail when concurrent workspace requests complete out of order', async () => {
    let resolveOld: (value: Response) => void = () => {}
    const current = templateDetail('2026-05-01T00:00:00Z')
    vi.stubGlobal('fetch', vi.fn()
      .mockImplementationOnce(() => new Promise<Response>(resolve => { resolveOld = resolve }))
      .mockResolvedValueOnce(jsonResponse({ template: current })))
    const store = useRenderTemplatesStore()
    const pending = store.fetchTemplateWorkspace('help.menu')
    await store.fetchTemplateWorkspace('help.menu')
    resolveOld(jsonResponse({ template: templateDetail() }))
    await pending
    expect(store.detailById['help.menu']).toEqual(current)
    expect(store.items[0]?.updated_at).toBe(current.updated_at)
    expect(store.workspaceLoading).toBe(false)
  })

  it('invalidates details on catalog refresh even when only preview data has changed', async () => {
    let resolveOld: (value: Response) => void = () => {}
    const detail = templateDetail()
    vi.stubGlobal('fetch', vi.fn()
      .mockImplementationOnce(() => new Promise<Response>(resolve => { resolveOld = resolve }))
      .mockResolvedValueOnce(jsonResponse({ items: [detail] })))
    const store = useRenderTemplatesStore()
    store.detailById = { 'help.menu': detail }
    const pending = store.fetchTemplateWorkspace('help.menu')
    await store.fetchTemplates()
    expect(store.detailById).toEqual({})
    resolveOld(jsonResponse({ template: detail }))
    await pending
    expect(store.detailById).toEqual({})
  })

  it('removes stale details and does not revive a removed template from an older request', async () => {
    let resolveDetail: (value: Response) => void = () => {}
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({ items: [templateDetail()] }))
      .mockImplementationOnce(() => new Promise<Response>(resolve => { resolveDetail = resolve }))
      .mockResolvedValueOnce(jsonResponse({ items: [] }))
    vi.stubGlobal('fetch', fetchMock)
    const store = useRenderTemplatesStore()
    await store.fetchTemplates()
    store.detailById = { 'help.menu': templateDetail() }
    const pending = store.fetchTemplateWorkspace('help.menu')
    await store.fetchTemplates()
    expect(store.items).toEqual([])
    expect(store.detailById).toEqual({})
    resolveDetail(jsonResponse({ template: templateDetail() }))
    await pending
    expect(store.items).toEqual([])
    expect(store.detailById).toEqual({})
  })
})
