import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import AdaptersPage from '@/views/protocols/AdaptersView.vue'
import { useAdaptersStore } from '@/stores/adapters'

describe('AdaptersPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    document.body.innerHTML = ''
  })

  function createTestRouter() {
    return createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/protocols', component: AdaptersPage },
        { path: '/protocols/:protocol', component: { template: '<div />' } },
      ],
    })
  }

  async function mountPage(adapters: unknown[]) {
    const router = createTestRouter()
    await router.push('/protocols')
    await router.isReady()
    const store = useAdaptersStore()
    vi.spyOn(store, 'refresh').mockImplementation(async () => {
      store.adapters = adapters as never
      return { adapters } as never
    })
    const wrapper = mount(AdaptersPage, { global: { plugins: [Antd, router] } })
    await flushPromises()
    return { wrapper, router }
  }

  it('separates configured adapters from ones that can still be added', async () => {
    const { wrapper } = await mountPage([
      { protocol: 'onebot11', display_name: 'OneBot11', configured: true, enabled: true, state: 'connected', summary: '已连接。' },
      { protocol: 'qqofficial', display_name: 'QQ 官方机器人', configured: false, enabled: false, state: 'idle', summary: '适配器未配置。' },
    ])

    // The configured one is listed as added; the unconfigured one is offered
    // as something to add rather than hidden.
    expect(wrapper.find('[data-testid="adapter-onebot11"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="adapter-add-qqofficial"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="adapter-add-onebot11"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('QQ 官方机器人')
  })

  it('opens the adapter configuration when one is selected', async () => {
    const { wrapper, router } = await mountPage([
      { protocol: 'qqofficial', display_name: 'QQ 官方机器人', configured: false, enabled: false, state: 'idle', summary: '适配器未配置。' },
    ])

    await wrapper.get('[data-testid="adapter-add-qqofficial"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/protocols/qqofficial')
  })

  it('reports a configured but disabled adapter as a choice, not a failure', async () => {
    const { wrapper } = await mountPage([
      { protocol: 'qqofficial', display_name: 'QQ 官方机器人', configured: true, enabled: false, state: 'idle', summary: '适配器已配置但未启用。' },
    ])

    const card = wrapper.get('[data-testid="adapter-qqofficial"]')
    expect(card.text()).toContain('未启用')
    // It must not borrow the connection vocabulary, which would read as broken.
    expect(card.text()).not.toContain('鉴权失败')
  })

  it('surfaces a load failure instead of showing an empty list', async () => {
    const router = createTestRouter()
    await router.push('/protocols')
    await router.isReady()
    const store = useAdaptersStore()
    vi.spyOn(store, 'refresh').mockImplementation(async () => {
      store.error = '暂时无法读取适配器列表。'
      throw new Error('boom')
    })
    const wrapper = mount(AdaptersPage, { global: { plugins: [Antd, router] } })
    await flushPromises()

    expect(wrapper.text()).toContain('暂时无法读取适配器列表。')
  })
})
