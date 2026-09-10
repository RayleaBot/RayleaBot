import { effectScope } from 'vue'
import { createPinia, disposePinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { collectionURL, createCollectionPager } from '@/lib/collection-pager'
import { usePluginsStore } from '@/stores/plugins'
import { useGovernanceStore } from '@/stores/governance'
import { useRenderTemplatesStore } from '@/stores/render-templates'
import type { PluginDetail, PluginSummary } from '@/types/api'

const plugin = (id: string): PluginSummary => ({ id, name: `插件 ${id}`, role: 'community', state: 'running', commands: [], command_groups: [], help: {} })
const json = (value: unknown) => new Response(JSON.stringify(value), { headers: { 'Content-Type': 'application/json' } })

describe('bounded collection pagination', () => {
  let pinia: ReturnType<typeof createPinia>
  beforeEach(() => { pinia = createPinia(); setActivePinia(pinia) })
  afterEach(() => { disposePinia(pinia); vi.unstubAllGlobals(); vi.useRealTimers() })

  it('loads only requested plugin pages and retains an off-page detail/name during refresh', async () => {
    const all = Array.from({ length: 205 }, (_, i) => plugin(`plugin-${String(i).padStart(3, '0')}`))
    const calls: string[] = []
    vi.stubGlobal('fetch', vi.fn(async (path: string) => {
      calls.push(path)
      const url = new URL(path, 'http://local.test')
      if (url.pathname !== '/api/plugins') return json({ plugin: { ...all[204], permissions: {}, webhooks: [] } })
      const offset = Number(url.searchParams.get('cursor') || 0)
      return json({ items: all.slice(offset, offset + 100), total: all.length, ...(offset + 100 < all.length ? { next_cursor: String(offset + 100) } : {}) })
    }))
    const store = usePluginsStore()
    await store.fetchList({})
    expect(calls).toEqual(['/api/plugins'])
    expect(store.items).toHaveLength(100)
    expect(store.nextCursor).toBe('100')
    await store.ensureDetail('plugin-204')
    expect(store.items).toHaveLength(100)
    await store.loadMore()
    expect(store.items).toHaveLength(200)
    const beforeRefresh = calls.length
    await store.refreshList()
    expect(calls.slice(beforeRefresh)).toEqual(['/api/plugins', '/api/plugins?cursor=100'])
    expect(store.nextCursor).toBe('200')
    expect(store.total).toBe(205)
    expect(store.detailsByPluginId['plugin-204']?.name).toBe('插件 plugin-204')
    expect(store.getPluginDisplayName('plugin-204')).toBe('插件 plugin-204')
  })

  it('does not overwrite a newer remote search with an old page or append response', async () => {
    let resolveOld!: (value: { items: string[]; total: number }) => void
    const apply = vi.fn()
    const scope = effectScope()
    const pager = scope.run(() => createCollectionPager({
      request: async (query) => query.query === 'old'
        ? new Promise<{ items: string[]; total: number }>(resolve => { resolveOld = resolve })
        : { items: ['found beyond the first page'], total: 1 },
      apply,
    }))!
    const stale = pager.load({ query: 'old' }).catch(() => undefined)
    await pager.load({ query: 'new' })
    resolveOld({ items: ['stale'], total: 1000 })
    await stale
    expect(apply).toHaveBeenCalledTimes(1)
    expect(pager.total.value).toBe(1)
    expect(pager.error.value).toBeNull()
    scope.stop()
  })

  it('keeps unfiltered governance entry_count when a remote filter has no matches', async () => {
    vi.stubGlobal('fetch', vi.fn(async path => {
      expect(path).toBe('/api/governance/whitelist?query=missing&entry_type=group')
      return json({ enabled: true, user_entries: [], group_entries: [], total: 0, entry_count: 250 })
    }))
    const store = useGovernanceStore()
    await store.fetchWhitelist(undefined, { query: 'missing', entry_type: 'group' })
    expect(store.whitelist?.total).toBe(0)
    expect(store.whitelist?.entry_count).toBe(250)
  })

  it('keeps template details outside a partial or filtered catalog', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => json({ items: [], total: 0 })))
    const store = useRenderTemplatesStore()
    const detail = { id: 'off-page', name: 'Deep link', source: { type: 'system' } } as never
    store.detailById = { 'off-page': detail }
    await store.fetchTemplates({ query: 'not-this-template' })
    expect(store.detailById['off-page']).toEqual(detail)
  })

  it('updates known plugin state without injecting an unloaded plugin into the visible page', () => {
    const store = usePluginsStore()
    const known = { ...plugin('off-page'), permissions: {}, webhooks: [] } as PluginDetail
    store.rememberSummaries([known])
    store.detailsByPluginId = { 'off-page': known }
    store.upsert({ id: 'off-page', state: 'disabled' })
    expect(store.items).toEqual([])
    expect(store.knownItems[0]?.state).toBe('disabled')
    expect(store.detailsByPluginId['off-page']?.name).toBe(known.name)
    expect(collectionURL('/api/plugins', { query: '  中文 query  ' }, '100')).toBe('/api/plugins?query=%E4%B8%AD%E6%96%87+query&cursor=100')
  })
})
