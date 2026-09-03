import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { usePluginStore } from '@/stores/plugin-store'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('plugin store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.unstubAllGlobals()
  })

  it('loads one source and forwards source, search and sort parameters', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      items: [{
        id: 'raylea.echo',
        name: 'Echo',
        summary: 'Echo messages',
        publisher: { id: 'rayleabot', name: 'RayleaBot' },
        repository_url: 'https://github.com/RayleaBot/plugin-echo',
        license: 'MIT',
        keywords: ['echo'],
        recommended: true,
        install_state: 'unpublished',
      }],
      total: 1,
      source: {
        id: 'official',
        name: 'RayleaBot 官方插件',
        url: 'https://plugins.example/catalog.json',
        official: true,
        cached: true,
        refreshed_at: '2026-08-04T00:00:00Z',
        entry_count: 4,
      },
    }))
    vi.stubGlobal('fetch', fetchMock)

    const store = usePluginStore()
    await store.fetchEntries({ sourceId: 'official', query: ' echo ', sort: 'name' })

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/plugin-store/plugins?source_id=official&query=echo&sort=name&limit=100',
      expect.any(Object),
    )
    expect(store.items.map((item) => item.id)).toEqual(['raylea.echo'])
    expect(store.source?.id).toBe('official')
  })

  it('passes the accepted inspection handle to the install endpoint', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ task_id: 'task-store-install' }, 202))
    vi.stubGlobal('fetch', fetchMock)

    const store = usePluginStore()
    await store.install('raylea.echo', {
      inspection_id: 'i'.repeat(64),
      package_sha256: 'a'.repeat(64),
      trusted_code_confirmed: false,
    })

    const [, request] = fetchMock.mock.calls[0]
    expect(JSON.parse(String(request.body))).toEqual({
      inspection_id: 'i'.repeat(64),
      package_sha256: 'a'.repeat(64),
      trusted_code_confirmed: false,
    })
    expect(store.installing['raylea.echo']).toBe(false)
  })
})
