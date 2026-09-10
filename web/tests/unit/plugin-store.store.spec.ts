import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'

import { usePluginStore } from '@/stores/plugin-store'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('plugin store', () => {
  afterEach(() => { vi.useRealTimers() })
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
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({ task_id: 'task-store-install' }, 202))
      .mockResolvedValueOnce(jsonResponse({ task_id: 'task-store-install', status: 'succeeded' }))
      .mockResolvedValueOnce(jsonResponse({ items: [] }))
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

  it('keeps installation pending until the task finishes and refreshes installed state', async () => {
    vi.useFakeTimers()
    let checks = 0
    const source = { id: 'official', name: '官方源', url: 'https://example.test/catalog', official: true, cached: true, entry_count: 1 }
    const fetchMock = vi.fn(async (url: string, request?: RequestInit) => {
      if (request?.method === 'POST') return jsonResponse({ task_id: 'task-install' }, 202)
      if (url.includes('/tasks/')) return jsonResponse({ task_id: 'task-install', status: ++checks === 1 ? 'running' : 'succeeded' })
      if (url === '/api/plugins') return jsonResponse({ items: [] })
      return jsonResponse({ items: [{ id: 'echo', installed_version: checks > 1 ? '2.0.0' : '1.0.0', install_state: checks > 1 ? 'installed' : 'update_available' }], total: 1, source })
    })
    vi.stubGlobal('fetch', fetchMock)
    const store = usePluginStore()
    await store.fetchEntries()
    const pending = store.install('echo', { inspection_id: 'fixture', package_sha256: 'fixture', trusted_code_confirmed: true })
    await flushPromises()
    expect(store.installing.echo).toBe(true)
    expect(store.items[0]?.installed_version).toBe('1.0.0')
    await vi.advanceTimersByTimeAsync(1000)
    await pending
    expect(store.installing.echo).toBe(false)
    expect(store.items[0]?.installed_version).toBe('2.0.0')
    expect(store.items[0]?.install_state).toBe('installed')
  })

  it('reports a failed task and releases installation state', async () => {
    vi.stubGlobal('fetch', vi.fn()
      .mockResolvedValueOnce(jsonResponse({ task_id: 'task-failed' }, 202))
      .mockResolvedValueOnce(jsonResponse({ task_id: 'task-failed', status: 'failed', error_code: 'plugin.internal_error' }))
      .mockResolvedValueOnce(jsonResponse({ items: [] })))
    const store = usePluginStore()
    await expect(store.install('echo', { inspection_id: 'fixture', package_sha256: 'fixture', trusted_code_confirmed: true })).rejects.toMatchObject({ code: 'plugin.internal_error' })
    expect(store.installing.echo).toBe(false)
  })

  it('loads cursor pages and rejects older source responses', async () => {
    let resolveOld: (value: Response) => void = () => {}
    const source = { id: 'official', name: '官方源', url: 'https://example.test/catalog', official: true, cached: true, entry_count: 101 }
    const first = Array.from({ length: 100 }, (_, i) => ({ id: `plugin-${i}` }))
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({ items: first, total: 101, next_cursor: '100', source }))
      .mockResolvedValueOnce(jsonResponse({ items: [{ id: 'last-plugin' }], total: 101, source }))
      .mockImplementationOnce(() => new Promise<Response>(resolve => { resolveOld = resolve }))
      .mockResolvedValueOnce(jsonResponse({ items: [{ id: 'new-source' }], total: 1, source: { ...source, id: 'custom' } }))
    vi.stubGlobal('fetch', fetchMock)
    const store = usePluginStore()
    await store.fetchEntries()
    await store.loadMore()
    expect(store.items).toHaveLength(101)
    expect(String(fetchMock.mock.calls[1]?.[0])).toContain('cursor=100')
    const old = store.fetchEntries()
    await store.fetchEntries({ sourceId: 'custom' })
    resolveOld(jsonResponse({ items: first, total: 101, source }))
    await old
    expect(store.items.map(item => item.id)).toEqual(['new-source'])
    expect(store.source?.id).toBe('custom')
  })

  it('does not keep a previous source cursor when loading a new source fails', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({ items: [{ id: 'old-source-plugin' }], total: 101, next_cursor: '100', source: { id: 'official' } }))
      .mockRejectedValueOnce(new Error('source unavailable'))
    vi.stubGlobal('fetch', fetchMock)
    const store = usePluginStore()
    await store.fetchEntries()
    await expect(store.fetchEntries({ sourceId: 'custom' })).rejects.toThrow('source unavailable')
    await store.loadMore()
    expect(store.items).toEqual([])
    expect(store.nextCursor).toBe('')
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })
})
