import { createPinia, getActivePinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AppPopover from '@/components/AppPopover.vue'
import AppInput from '@/components/AppInput.vue'
import AppCheckbox from '@/components/AppCheckbox.vue'
import AppCollectionPagination from '@/components/AppCollectionPagination.vue'
import PluginPicker from '@/components/plugins/PluginPicker.vue'
import ManagementLogRow from '@/components/logs/ManagementLogRow.vue'
import { apiRequest } from '@/lib/http'
import { usePluginsStore } from '@/stores/plugins'
import type { PluginSummary } from '@/types/api'

vi.mock('@/lib/http', async importOriginal => ({ ...await importOriginal<typeof import('@/lib/http')>(), apiRequest: vi.fn() }))

function plugin(id: string, name = id): PluginSummary {
  return { id, name, role: 'community', state: 'running', commands: [], command_groups: [], command_conflicts: [], help: {} }
}

describe('paginated plugin consumers', () => {
  beforeEach(() => { setActivePinia(createPinia()); vi.clearAllMocks(); document.body.innerHTML = '' })
  afterEach(() => vi.useRealTimers())

  it('keeps an off-page selection and loads more options only on demand', async () => {
    const mainPage = plugin('main-page', '主列表')
    const store = usePluginsStore()
    store.items = [mainPage]
    vi.mocked(apiRequest).mockImplementation(async path => {
      if (path === '/api/plugins/selected') return { plugin: { ...plugin('selected', '已选插件'), permissions: {}, webhooks: [] } } as never
      if (path.includes('cursor=1')) return { items: [plugin('second', '第二页插件')], total: 2 } as never
      return { items: [plugin('first', '第一页插件')], total: 2, next_cursor: '1' } as never
    })
    const wrapper = mount(PluginPicker, { attachTo: document.body, props: { modelValue: ['selected'], multiple: true }, global: { plugins: [getActivePinia()!] } })
    await flushPromises()
    expect(wrapper.text()).toContain('已选插件')
    expect(apiRequest).toHaveBeenCalledTimes(1)
    await wrapper.getComponent(AppPopover).vm.$emit('update:open', true)
    await flushPromises()
    expect(apiRequest).toHaveBeenCalledTimes(2)
    expect(wrapper.getComponent(AppCollectionPagination).props('nextCursor')).toBe('1')
    expect(document.body.textContent).not.toContain('第二页插件')
    await wrapper.getComponent(AppCollectionPagination).vm.$emit('more')
    await flushPromises()
    expect(document.body.textContent).toContain('第二页插件')
    const second = wrapper.findAllComponents(AppCheckbox).find(option => option.text().includes('第二页插件'))!
    await second.vm.$emit('update:modelValue', true)
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([['selected', 'second']])
    expect(store.items).toEqual([mainPage])
    wrapper.unmount()
  })

  it('rejects stale search responses and cancels a delayed search when closed', async () => {
    vi.useFakeTimers()
    let releaseOld!: (value: unknown) => void
    vi.mocked(apiRequest).mockImplementation(async path => {
      if (path.includes('query=old')) return await new Promise(resolve => { releaseOld = resolve }) as never
      if (path.includes('query=new')) return { items: [plugin('new', '新搜索插件')], total: 1 } as never
      return { items: [], total: 0 } as never
    })
    const wrapper = mount(PluginPicker, { attachTo: document.body, props: { modelValue: [], multiple: true } })
    await wrapper.getComponent(AppPopover).vm.$emit('update:open', true)
    await flushPromises()
    const search = wrapper.getComponent(AppInput)
    await search.vm.$emit('update:modelValue', 'old')
    await vi.advanceTimersByTimeAsync(250)
    await search.vm.$emit('update:modelValue', 'new')
    await vi.advanceTimersByTimeAsync(250)
    releaseOld({ items: [plugin('old', '旧搜索插件')], total: 1 })
    await flushPromises()
    expect(document.body.textContent).toContain('新搜索插件')
    expect(document.body.textContent).not.toContain('旧搜索插件')
    const requests = vi.mocked(apiRequest).mock.calls.length
    await search.vm.$emit('update:modelValue', 'closed-query')
    await wrapper.getComponent(AppPopover).vm.$emit('update:open', false)
    await vi.advanceTimersByTimeAsync(300)
    expect(apiRequest).toHaveBeenCalledTimes(requests)
    wrapper.unmount()
  })

  it('resolves only the visible log plugin and keeps its name after a page change', async () => {
    vi.mocked(apiRequest).mockResolvedValue({ plugin: { ...plugin('off-page', '历史插件'), permissions: {}, webhooks: [] } })
    const store = usePluginsStore()
    const wrapper = mount(ManagementLogRow, { props: { selected: false, item: { level: 'info', source: 'plugin', timestamp: '2026-09-10T00:00:00Z', message: 'fixture', plugin_id: 'off-page' } } })
    await flushPromises()
    expect(wrapper.text()).toContain('历史插件')
    expect(vi.mocked(apiRequest).mock.calls.map(call => call[0])).toEqual(['/api/plugins/off-page'])
    store.items = [plugin('different-page', '另一个插件')]
    await flushPromises()
    expect(wrapper.text()).toContain('历史插件')
    wrapper.unmount()
  })
})
