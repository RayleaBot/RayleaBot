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

  it('loads the verified catalog and forwards search and sort parameters', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      items: [{
        id: 'raylea.echo',
        name: 'Echo',
        summary: 'Echo messages',
        publisher: { id: 'rayleabot', name: 'RayleaBot', verified: true },
        repository_url: 'https://github.com/RayleaBot/plugin-echo',
        license: 'MIT',
        keywords: ['echo'],
        recommended: true,
        install_state: 'unpublished',
      }],
      total: 1,
      catalog: {
        source: 'embedded',
        verified: true,
        generated_at: '2026-08-04T00:00:00Z',
        entry_count: 4,
      },
    }))
    vi.stubGlobal('fetch', fetchMock)

    const store = usePluginStore()
    await store.fetchEntries({ query: ' echo ', sort: 'name' })

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/plugin-store/plugins?query=echo&sort=name&limit=100',
      expect.any(Object),
    )
    expect(store.items.map((item) => item.id)).toEqual(['raylea.echo'])
    expect(store.hasVerifiedCatalog).toBe(true)
  })

  it('requires the explicit trusted-code confirmation in every install request', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ task_id: 'task-store-install' }, 202))
    vi.stubGlobal('fetch', fetchMock)

    const store = usePluginStore()
    await store.install('raylea.echo', '0.2.0')

    const [, request] = fetchMock.mock.calls[0]
    expect(JSON.parse(String(request.body))).toEqual({
      version: '0.2.0',
      trusted_code_confirmed: true,
    })
    expect(store.installing['raylea.echo']).toBe(false)
  })
})
