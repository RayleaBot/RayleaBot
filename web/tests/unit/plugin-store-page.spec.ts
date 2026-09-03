import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import PluginStoreView from '@/views/plugins/PluginStoreView.vue'
import { usePluginStore } from '@/stores/plugin-store'

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
  repository_url: 'https://github.com/RayleaBot/plugin-echo',
  license: 'MIT',
  keywords: ['echo'],
  recommended: true,
  latest_release: {
    version: '0.4.0',
    published_at: '2026-09-03T00:00:00Z',
    min_core_version: '0.4.0',
    compatible: true,
    asset_available: true,
  },
  install_state: 'available' as const,
}

describe('PluginStoreView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('shows inspected permissions before accepting a first installation', async () => {
    const store = usePluginStore()
    store.items = [echoPlugin]
    store.sources = [officialSource]
    store.source = officialSource
    store.total = 1
    vi.spyOn(store, 'fetchSources').mockResolvedValue(store.sources)
    vi.spyOn(store, 'fetchEntries').mockResolvedValue({ items: store.items, total: 1, source: officialSource })
    vi.spyOn(store, 'refreshSource').mockResolvedValue(officialSource)
    vi.spyOn(store, 'inspect').mockImplementation(async () => {
      store.installing = { 'raylea.echo': true }
      return {
      inspection: {
        inspection_id: 'i'.repeat(64),
        expires_at: '2026-09-03T00:15:00Z',
        package_sha256: 'a'.repeat(64),
        source: { source_type: 'catalog', source: 'official' },
        plugin: { id: 'raylea.echo', name: 'Echo', version: '0.4.0', author: 'raylea', license: 'MIT', source_label: 'RayleaBot 官方插件' },
        permissions: { 'message.send': true },
        target_platform: 'windows-x64',
        backend: { entry: 'bin/raylea.echo.exe', path: 'bin/raylea.echo.exe', size: 1024 },
        ui: { enabled: false, file_count: 0 },
        artifact: { valid: true, artifact_version: '2', file_count: 4 },
      },
      confirmation_required: true,
        confirmation_reasons: ['first_install'],
      }
    })
    const install = vi.spyOn(store, 'install').mockResolvedValue({ task_id: 'task-store-install' })

    const wrapper = mount(PluginStoreView, { global: { plugins: [Antd] } })
    await flushPromises()
    await wrapper.get('[data-testid="plugin-store-install-raylea.echo"]').trigger('click')
    await flushPromises()

    expect(install).not.toHaveBeenCalled()
    expect(store.installing['raylea.echo']).toBe(false)
    expect(document.body.textContent).toContain('message.send')
    const okButton = document.body.querySelector('.ant-modal .ant-btn-primary') as HTMLButtonElement
    okButton.click()
    await flushPromises()

    expect(install).toHaveBeenCalledWith('raylea.echo', {
      inspection_id: 'i'.repeat(64),
      package_sha256: 'a'.repeat(64),
      trusted_code_confirmed: true,
    })
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

    const wrapper = mount(PluginStoreView, { global: { plugins: [Antd] } })
    await flushPromises()
    expect(refresh).toHaveBeenCalledWith('official')
    expect(store.items).toEqual([echoPlugin])

    await wrapper.get('[data-testid="plugin-store-refresh"]').trigger('click')
    await flushPromises()
    expect(refresh).toHaveBeenCalledTimes(2)
    expect(store.items).toEqual([echoPlugin])
  })
})
