import { createPinia, getActivePinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import PluginStoreView from '@/views/plugins/PluginStoreView.vue'
import { notifyError } from '@/adapter/feedback'
import { usePluginStore } from '@/stores/plugin-store'
import type { PluginStoreDependency, PluginStoreEntry } from '@/types/api'

vi.mock('@/adapter/feedback', () => ({
  notifyError: vi.fn(),
  notifySuccess: vi.fn(),
  notifyWarning: vi.fn(),
}))

const officialSource = {
  id: 'official',
  name: 'RayleaBot 官方插件',
  url: 'https://plugins.example/catalog.json',
  official: true,
  cached: true,
  refreshed_at: '2026-09-03T00:00:00Z',
  entry_count: 1,
}

const echoPlugin = {
  id: 'raylea.echo',
  name: 'Echo',
  summary: 'Echo messages',
  publisher: { id: 'rayleabot', name: 'RayleaBot' },
  repository_url: 'https://github.com/RayleaBot/echo',
  license: 'MIT',
  keywords: ['echo'],
  recommended: true,
  latest_release: {
    version: '0.4.0',
    published_at: '2026-09-03T00:00:00Z',
    min_core_version: '0.4.0',
    compatible: true,
    asset_available: true,
    dependencies: [] as PluginStoreDependency[],
  },
  install_state: 'available' as const,
  confirmation_reasons: ['first_install' as const],
}

const accounts: PluginStoreDependency = { id: 'raylea.mihoyo-accounts', name: '米游社账号', requirement: 'required', state: 'installable' }
const assets: PluginStoreDependency = { id: 'raylea.panel-assets', name: '面板素材', requirement: 'recommended', reason: '离线面板图', state: 'installable' }

function withDependencies(...dependencies: PluginStoreDependency[]): PluginStoreEntry {
  return { ...echoPlugin, id: 'raylea.genshin', name: '原神', latest_release: { ...echoPlugin.latest_release, dependencies } }
}

function mountStore(items: PluginStoreEntry[]) {
  const store = usePluginStore()
  store.items = items
  store.sources = [officialSource]
  store.source = officialSource
  store.total = items.length
  vi.spyOn(store, 'fetchSources').mockResolvedValue(store.sources)
  vi.spyOn(store, 'fetchEntries').mockResolvedValue({ items: store.items, total: items.length, source: officialSource })
  vi.spyOn(store, 'refreshSource').mockResolvedValue(officialSource)
  const wrapper = mount(PluginStoreView, { global: { plugins: [getActivePinia()!] } })
  return { store, wrapper }
}

function confirmButton() {
  return document.body.querySelector('[data-testid="plugin-store-install-confirm"]') as HTMLButtonElement
}

describe('PluginStoreView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('confirms trusted code before a first installation', async () => {
    const store = usePluginStore()
    store.items = [echoPlugin]
    store.sources = [officialSource]
    store.source = officialSource
    store.total = 1
    vi.spyOn(store, 'fetchSources').mockResolvedValue(store.sources)
    vi.spyOn(store, 'fetchEntries').mockResolvedValue({ items: store.items, total: 1, source: officialSource })
    vi.spyOn(store, 'refreshSource').mockResolvedValue(officialSource)
    const install = vi.spyOn(store, 'install').mockResolvedValue({ task_id: 'task-store-install' })

    const wrapper = mount(PluginStoreView, { global: { plugins: [getActivePinia()!] } })
    await flushPromises()
    await wrapper.get('[data-testid="plugin-store-install-raylea.echo"]').trigger('click')
    await flushPromises()

    expect(install).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain('首次安装')
    const okButton = document.body.querySelector('[role=dialog] button[data-variant=default]') as HTMLButtonElement
    okButton.click()
    await flushPromises()

    expect(install).toHaveBeenCalledWith('raylea.echo', {
      source_id: 'official',
      trusted_code_confirmed: true,
    })
  })

  it('installs the chosen prerequisite plugins before the plugin that needs them', async () => {
    const { store, wrapper } = mountStore([withDependencies(accounts, assets)])
    const install = vi.spyOn(store, 'install').mockResolvedValue({ task_id: 'task-store-install' })
    await flushPromises()

    await wrapper.get('[data-testid="plugin-store-install-raylea.genshin"]').trigger('click')
    await flushPromises()
    expect(install).not.toHaveBeenCalled()
    // A required prerequisite cannot be unticked; the recommended one can.
    const requiredBox = document.body.querySelector('[data-testid="plugin-store-dependency-raylea.mihoyo-accounts"] button') as HTMLButtonElement
    expect(requiredBox.disabled).toBe(true)
    ;(document.body.querySelector('[data-testid="plugin-store-dependency-raylea.panel-assets"] button') as HTMLButtonElement).click()
    await flushPromises()

    confirmButton().click()
    await flushPromises()

    expect(install.mock.calls).toEqual([
      ['raylea.mihoyo-accounts', { source_id: 'official', trusted_code_confirmed: true }],
      ['raylea.genshin', { source_id: 'official', trusted_code_confirmed: true }],
    ])
  })

  it('cannot install while a required prerequisite is unavailable from the source', async () => {
    const { store, wrapper } = mountStore([withDependencies({ ...accounts, state: 'unavailable' })])
    const install = vi.spyOn(store, 'install').mockResolvedValue({ task_id: 'task-store-install' })
    await flushPromises()

    await wrapper.get('[data-testid="plugin-store-install-raylea.genshin"]').trigger('click')
    await flushPromises()

    expect(confirmButton().disabled).toBe(true)
    confirmButton().click()
    await flushPromises()
    expect(install).not.toHaveBeenCalled()
  })

  it('stops before the plugin when a prerequisite fails to install', async () => {
    const { store, wrapper } = mountStore([withDependencies(accounts)])
    const install = vi.spyOn(store, 'install').mockRejectedValue(new Error('download failed'))
    await flushPromises()

    await wrapper.get('[data-testid="plugin-store-install-raylea.genshin"]').trigger('click')
    await flushPromises()
    confirmButton().click()
    await flushPromises()

    expect(install).toHaveBeenCalledTimes(1)
    expect(install).toHaveBeenCalledWith('raylea.mihoyo-accounts', expect.anything())
    expect(notifyError).toHaveBeenCalledWith(expect.stringContaining('米游社账号'))
  })

  it('names why a release cannot be installed instead of one generic incompatibility', async () => {
    const store = usePluginStore()
    const release = echoPlugin.latest_release
    store.items = [
      { ...echoPlugin, id: 'needs-core', name: 'Needs Core', install_state: 'incompatible', latest_release: { ...release, min_core_version: '0.9.0', compatible: false, incompatible_reason: 'core_version_too_old' } },
      { ...echoPlugin, id: 'unknown-core', name: 'Unknown Core', install_state: 'incompatible', latest_release: { ...release, compatible: false, incompatible_reason: 'core_version_unknown' } },
      { ...echoPlugin, id: 'no-asset', name: 'No Asset', install_state: 'incompatible', latest_release: { ...release, asset_available: false } },
      { ...echoPlugin, id: 'blocked-update', name: 'Blocked Update', install_state: 'installed', installed_version: '0.3.0', latest_release: { ...release, asset_available: false } },
      { ...echoPlugin, id: 'older-in-store', name: 'Older In Store', install_state: 'installed', installed_version: '0.5.0', latest_release: { ...release, asset_available: false } },
    ]
    store.sources = [officialSource]
    store.source = officialSource
    store.total = 5
    vi.spyOn(store, 'fetchSources').mockResolvedValue(store.sources)
    vi.spyOn(store, 'fetchEntries').mockResolvedValue({ items: store.items, total: 5, source: officialSource })
    vi.spyOn(store, 'refreshSource').mockResolvedValue(officialSource)

    const wrapper = mount(PluginStoreView, { global: { plugins: [getActivePinia()!] } })
    await flushPromises()

    const cards = wrapper.findAll('.store-plugin-card')
    expect(cards[0]!.text()).toContain('需要 RayleaBot v0.9.0 或更高')
    // An unknown RayleaBot version is not an old one; asking for a newer RayleaBot would mislead.
    expect(cards[1]!.text()).toContain('无法确认当前 RayleaBot 版本')
    expect(cards[1]!.text()).not.toContain('或更高')
    expect(cards[2]!.text()).toContain('没有适用于本机平台的安装包')
    expect(cards[3]!.text()).toContain('商店新版本暂不能更新：没有适用于本机平台的安装包')
    // A store release older than the installed version is no update, so nothing claims one is blocked.
    expect(cards[4]!.text()).not.toContain('暂不能更新')
  })

  it('opens the plugin repository as an external link', async () => {
    const store = usePluginStore()
    store.items = [echoPlugin]
    store.sources = [officialSource]
    store.source = officialSource
    store.total = 1
    vi.spyOn(store, 'fetchSources').mockResolvedValue(store.sources)
    vi.spyOn(store, 'fetchEntries').mockResolvedValue({ items: store.items, total: 1, source: officialSource })
    vi.spyOn(store, 'refreshSource').mockResolvedValue(officialSource)

    const wrapper = mount(PluginStoreView, { global: { plugins: [getActivePinia()!] } })
    await flushPromises()

    const link = wrapper.get('a[aria-label="打开源码仓库"]')
    expect(link.attributes('href')).toBe('https://github.com/RayleaBot/echo')
    expect(link.attributes('target')).toBe('_blank')
    expect(link.attributes('rel')).toBe('noopener noreferrer')
  })

  it('keeps cached entries when refreshing the selected source fails', async () => {
    const store = usePluginStore()
    store.items = [echoPlugin]
    store.sources = [officialSource]
    store.source = officialSource
    store.total = 1
    vi.spyOn(store, 'fetchSources').mockResolvedValue(store.sources)
    vi.spyOn(store, 'fetchEntries').mockResolvedValue({ items: store.items, total: 1, source: officialSource })
    const refresh = vi.spyOn(store, 'refreshSource').mockRejectedValue(new Error('offline'))

    const wrapper = mount(PluginStoreView, { global: { plugins: [getActivePinia()!] } })
    await flushPromises()
    expect(refresh).toHaveBeenCalledWith('official')
    expect(wrapper.findAll('.store-plugin-card')).toHaveLength(1)

    await wrapper.get('[data-testid="plugin-store-refresh"]').trigger('click')
    await flushPromises()
    expect(refresh).toHaveBeenCalledTimes(2)
    expect(wrapper.findAll('.store-plugin-card')).toHaveLength(1)
  })

  it('searches as the query is typed, once the typing pauses', async () => {
    vi.useFakeTimers()
    try {
      const store = usePluginStore()
      store.items = [echoPlugin]
      store.sources = [officialSource]
      store.source = officialSource
      store.total = 1
      vi.spyOn(store, 'fetchSources').mockResolvedValue(store.sources)
      const fetchEntries = vi.spyOn(store, 'fetchEntries').mockResolvedValue({ items: store.items, total: 1, source: officialSource })
      vi.spyOn(store, 'refreshSource').mockResolvedValue(officialSource)
      const wrapper = mount(PluginStoreView, { global: { plugins: [getActivePinia()!] } })
      await flushPromises()
      fetchEntries.mockClear()

      await wrapper.get('.store-search input').setValue('ec')
      await wrapper.get('.store-search input').setValue('echo')
      expect(fetchEntries).not.toHaveBeenCalled()
      await vi.advanceTimersByTimeAsync(300)
      expect(fetchEntries).toHaveBeenCalledTimes(1)
      expect(fetchEntries).toHaveBeenLastCalledWith(expect.objectContaining({ query: 'echo' }))
      wrapper.unmount()
    } finally {
      vi.useRealTimers()
    }
  })

  it('keeps the loaded catalog on screen while it reloads', async () => {
    const store = usePluginStore()
    store.items = [echoPlugin]
    store.sources = [officialSource]
    store.source = officialSource
    store.total = 1
    vi.spyOn(store, 'fetchSources').mockResolvedValue(store.sources)
    vi.spyOn(store, 'fetchEntries').mockResolvedValue({ items: store.items, total: 1, source: officialSource })
    vi.spyOn(store, 'refreshSource').mockResolvedValue(officialSource)

    const wrapper = mount(PluginStoreView, { global: { plugins: [getActivePinia()!] } })
    await flushPromises()

    // A reload of the same catalog, as on returning to the page, must not swap the cards for a skeleton.
    store.loading = true
    await flushPromises()
    expect(wrapper.findAll('.store-plugin-card')).toHaveLength(1)
    expect(wrapper.find('.app-skeleton-card').exists()).toBe(false)

    // With nothing loaded yet the skeleton stands in for the cards.
    store.items = []
    await flushPromises()
    expect(wrapper.find('.app-skeleton-card').exists()).toBe(true)
    expect(wrapper.find('.store-plugin-card').exists()).toBe(false)
  })
})
