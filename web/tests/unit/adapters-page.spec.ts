import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import AdaptersPage from '@/views/protocols/AdaptersView.vue'
import { useAdaptersStore } from '@/stores/adapters'
import { useConfigStore } from '@/stores/config'

const AVAILABLE_PROTOCOLS = [
  { protocol: 'onebot11', display_name: 'OneBot11', description: '连接实现 OneBot11 的第三方客户端。' },
  { protocol: 'qqofficial', display_name: 'QQ 官方机器人', description: '连接 QQ 开放平台的官方机器人。' },
]

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
        { path: '/protocols/:protocol/:adapterId', component: { template: '<div />' } },
      ],
    })
  }

  async function mountPage(adapters: unknown[], configAdapters: unknown[] = []) {
    const router = createTestRouter()
    await router.push('/protocols')
    await router.isReady()

    const store = useAdaptersStore()
    vi.spyOn(store, 'refresh').mockImplementation(async () => {
      store.adapters = adapters as never
      store.availableProtocols = AVAILABLE_PROTOCOLS as never
      return { adapters, available_protocols: AVAILABLE_PROTOCOLS } as never
    })

    const configStore = useConfigStore()
    configStore.document = { schema_version: '4', adapters: configAdapters } as never
    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined as never)
    const saveConfig = vi.spyOn(configStore, 'saveConfig')
      .mockResolvedValue({ restart_required: false } as never)

    const wrapper = mount(AdaptersPage, { global: { plugins: [Antd, router] } })
    await flushPromises()
    return { wrapper, router, saveConfig }
  }

  it('lists the configured instances and offers every protocol to add', async () => {
    const { wrapper } = await mountPage([
      { id: 'onebot11', protocol: 'onebot11', display_name: 'OneBot11', enabled: true, state: 'connected', summary: '已连接。' },
    ])

    expect(wrapper.find('[data-testid="adapter-onebot11"]').exists()).toBe(true)
    // A protocol stays addable after one instance exists: several instances of
    // one protocol are allowed, each with its own credentials.
    expect(wrapper.find('[data-testid="adapter-add-onebot11"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="adapter-add-qqofficial"]').exists()).toBe(true)
  })

  it('keeps two instances of one protocol apart in the listing', async () => {
    const { wrapper } = await mountPage([
      { id: 'onebot11', protocol: 'onebot11', display_name: 'OneBot11', enabled: true, state: 'connected', summary: '已连接。' },
      { id: 'second-bot', protocol: 'onebot11', display_name: 'OneBot11（second-bot）', enabled: false, state: 'stopped', summary: '适配器已配置但未启用。' },
    ])

    expect(wrapper.find('[data-testid="adapter-onebot11"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="adapter-second-bot"]').text()).toContain('second-bot')
  })

  it('adds an instance to the config and opens its settings', async () => {
    const { wrapper, router, saveConfig } = await mountPage([], [
      { id: 'onebot11', type: 'onebot11', enabled: true, onebot11: {} },
    ])

    await wrapper.get('[data-testid="adapter-add-onebot11"]').trigger('click')
    await flushPromises()

    expect(saveConfig).toHaveBeenCalledTimes(1)
    const saved = saveConfig.mock.calls[0][0] as unknown as Record<string, unknown>
    const instances = saved.adapters as Array<Record<string, unknown>>
    // The existing instance keeps its identifier, and the new one is numbered
    // rather than colliding with it.
    expect(instances.map((instance) => instance.id)).toEqual(['onebot11', 'onebot11-2'])
    expect(instances[1].enabled).toBe(false)
    expect(instances[1].onebot11).toHaveProperty('reverse_ws')
    expect(router.currentRoute.value.path).toBe('/protocols/onebot11/onebot11-2')
  })

  it('removes only the instance the operator chose', async () => {
    const { wrapper, saveConfig } = await mountPage(
      [
        { id: 'onebot11', protocol: 'onebot11', display_name: 'OneBot11', enabled: true, state: 'connected', summary: '已连接。' },
        { id: 'second-bot', protocol: 'onebot11', display_name: 'OneBot11（second-bot）', enabled: false, state: 'stopped', summary: '未启用。' },
      ],
      [
        { id: 'onebot11', type: 'onebot11', enabled: true, onebot11: {} },
        { id: 'second-bot', type: 'onebot11', enabled: false, onebot11: {} },
      ],
    )

    // Removal is behind a confirmation, so the test confirms rather than
    // asserting on the popover's own markup.
    const popconfirms = wrapper.findAllComponents({ name: 'APopconfirm' })
    expect(popconfirms).toHaveLength(2)
    await popconfirms[1].vm.$emit('confirm')
    await flushPromises()

    expect(saveConfig).toHaveBeenCalledTimes(1)
    const saved = saveConfig.mock.calls[0][0] as unknown as Record<string, unknown>
    expect((saved.adapters as Array<Record<string, unknown>>).map((instance) => instance.id)).toEqual(['onebot11'])
  })

  it('reports a configured but disabled adapter as a choice, not a failure', async () => {
    const { wrapper } = await mountPage([
      { id: 'qq-official', protocol: 'qqofficial', display_name: 'QQ 官方机器人', enabled: false, state: 'idle', summary: '适配器已配置但未启用。' },
    ])

    const card = wrapper.get('[data-testid="adapter-qq-official"]')
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
    const configStore = useConfigStore()
    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined as never)
    const wrapper = mount(AdaptersPage, { global: { plugins: [Antd, router] } })
    await flushPromises()

    expect(wrapper.text()).toContain('暂时无法读取适配器列表。')
  })
})
