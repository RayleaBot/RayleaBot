import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { notifyError, notifySuccess } from '@/adapter/feedback'
import QQOfficialPage from '@/views/protocols/QQOfficialView.vue'
import { useAdaptersStore } from '@/stores/adapters'
import { useConfigStore } from '@/stores/config'
import { createConfigDocumentFixture } from './config-document.fixture'

vi.mock('@/adapter/feedback', () => ({
  notifyError: vi.fn(),
  notifySuccess: vi.fn(),
  useToastFeedback: vi.fn(),
}))

describe('QQOfficialPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    document.body.innerHTML = ''
    vi.mocked(notifySuccess).mockClear()
    vi.mocked(notifyError).mockClear()
  })

  const ADAPTER_ID = 'qq-official'

  async function mountPage(qqOfficial?: Record<string, unknown>) {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/protocols/qqofficial/:adapterId', component: QQOfficialPage }],
    })
    await router.push(`/protocols/qqofficial/${ADAPTER_ID}`)
    await router.isReady()

    const configStore = useConfigStore()
    const doc = createConfigDocumentFixture() as Record<string, unknown>
    if (qqOfficial) {
      doc.adapters = [
        ...(doc.adapters as unknown[]),
        { id: ADAPTER_ID, type: 'qqofficial', enabled: false, qqofficial: qqOfficial },
      ]
    }
    vi.spyOn(configStore, 'fetchConfig').mockImplementation(async () => {
      configStore.document = doc as never
    })
    const saveConfig = vi.spyOn(configStore, 'saveConfig')
      .mockResolvedValue({ restart_required: false } as never)

    const adaptersStore = useAdaptersStore()
    vi.spyOn(adaptersStore, 'refresh').mockImplementation(async () => {
      adaptersStore.adapters = [{
        id: ADAPTER_ID, protocol: 'qqofficial', display_name: 'QQ 官方机器人',
        enabled: false, state: 'idle', summary: '适配器未配置。',
      }] as never
      return { adapters: adaptersStore.adapters, available_protocols: [] } as never
    })

    const wrapper = mount(QQOfficialPage, { attachTo: document.body, global: { plugins: [Antd, router] } })
    await flushPromises()
    return { wrapper, configStore, saveConfig }
  }

  function savedAdapter(saved: Record<string, unknown>) {
    const instances = saved.adapters as Array<Record<string, unknown>>
    return instances.find((instance) => instance.id === ADAPTER_ID) as Record<string, Record<string, unknown>>
  }

  it('shows the adapter status alongside its configuration', async () => {
    const { wrapper } = await mountPage()
    expect(wrapper.find('[data-testid="qq-adapter-status"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('适配器未配置。')
  })

  it('edits its own instance without disturbing the rest of the config', async () => {
    const { wrapper, configStore, saveConfig } = await mountPage({
      enabled: false, app_id: '', app_secret: '', intents: [], sandbox: false,
    })

    await wrapper.get('[data-testid="qq-app-id"]').setValue('100000001')
    await flushPromises()
    await wrapper.get('[data-testid="qq-save"]').trigger('click')
    await flushPromises()

    expect(saveConfig).toHaveBeenCalledTimes(1)
    const saved = saveConfig.mock.calls[0][0] as unknown as Record<string, unknown>
    expect(savedAdapter(saved).qqofficial.app_id).toBe('100000001')
    // The OneBot instance the fixture configures is a different adapter, so an
    // edit here must not reach into it.
    const untouched = (saved.adapters as Array<Record<string, unknown>>)[0]
    expect(untouched.id).toBe('onebot11')
    expect(untouched.onebot11).toEqual(
      ((configStore.document as unknown as Record<string, unknown>).adapters as Array<Record<string, unknown>>)[0].onebot11,
    )
    // Every other section must survive an adapter edit untouched.
    expect(saved.server).toEqual((configStore.document as unknown as Record<string, unknown>).server)
    expect(notifySuccess).toHaveBeenCalled()
  })

  it('keeps save disabled until something actually changes', async () => {
    const { wrapper } = await mountPage({
      enabled: false, app_id: '', app_secret: '', intents: [], sandbox: false,
    })
    expect(wrapper.get('[data-testid="qq-save"]').attributes('disabled')).toBeDefined()
  })
})
