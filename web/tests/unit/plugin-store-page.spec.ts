import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import PluginStoreView from '@/views/plugins/PluginStoreView.vue'
import { usePluginStore } from '@/stores/plugin-store'

vi.mock('@/adapter/feedback', () => ({
  notifyError: vi.fn(),
  notifySuccess: vi.fn(),
}))

describe('PluginStoreView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('does not install executable code until the confirmation dialog is accepted', async () => {
    const store = usePluginStore()
    store.items = [{
      id: 'raylea.echo',
      name: 'Echo',
      summary: 'Echo messages',
      publisher: { id: 'rayleabot', name: 'RayleaBot', verified: true },
      repository_url: 'https://github.com/RayleaBot/plugin-echo',
      license: 'MIT',
      keywords: ['echo'],
      recommended: true,
      latest_release: {
        version: '0.2.0',
        published_at: '2026-08-04T00:00:00Z',
        min_core_version: '0.1.0',
        compatible: true,
        asset_available: true,
        yanked: false,
      },
      install_state: 'available',
    }]
    store.total = 1
    vi.spyOn(store, 'fetchEntries').mockResolvedValue({
      items: store.items,
      total: 1,
      catalog: {
        source: 'embedded',
        verified: true,
        generated_at: '2026-08-04T00:00:00Z',
        entry_count: 1,
      },
    })
    const install = vi.spyOn(store, 'install').mockResolvedValue({ task_id: 'task-store-install' })

    const wrapper = mount(PluginStoreView, { global: { plugins: [Antd] } })
    await flushPromises()
    await wrapper.get('[data-testid="plugin-store-install-raylea.echo"]').trigger('click')

    expect(install).not.toHaveBeenCalled()
    const dialog = document.body.querySelector('.ant-modal')
    expect(dialog).not.toBeNull()
    const okButton = document.body.querySelector('.ant-modal .ant-btn-primary') as HTMLButtonElement
    okButton.click()
    await flushPromises()

    expect(install).toHaveBeenCalledWith('raylea.echo', '0.2.0')
  })

  it('refreshes an embedded catalog while retaining it as the offline fallback', async () => {
    const store = usePluginStore()
    store.catalog = {
      source: 'embedded',
      verified: true,
      generated_at: '2026-08-04T00:00:00Z',
      entry_count: 4,
    }
    vi.spyOn(store, 'fetchEntries').mockResolvedValue({
      items: [],
      total: 0,
      catalog: store.catalog,
    })
    const refresh = vi.spyOn(store, 'refreshCatalog').mockRejectedValue(new Error('offline'))

    mount(PluginStoreView, { global: { plugins: [Antd] } })
    await flushPromises()

    expect(refresh).toHaveBeenCalledOnce()
    expect(store.catalog?.source).toBe('embedded')
  })
})
