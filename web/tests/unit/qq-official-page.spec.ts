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

  async function mountPage(qqOfficial?: Record<string, unknown>) {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/protocols/qqofficial', component: QQOfficialPage }],
    })
    await router.push('/protocols/qqofficial')
    await router.isReady()

    const configStore = useConfigStore()
    const doc = createConfigDocumentFixture() as Record<string, unknown>
    if (qqOfficial) {
      doc.qq_official = qqOfficial
    }
    vi.spyOn(configStore, 'fetchConfig').mockImplementation(async () => {
      configStore.document = doc as never
    })
    const saveConfig = vi.spyOn(configStore, 'saveConfig')
      .mockResolvedValue({ restart_required: false } as never)

    const adaptersStore = useAdaptersStore()
    vi.spyOn(adaptersStore, 'refresh').mockImplementation(async () => {
      adaptersStore.adapters = [{
        protocol: 'qqofficial', display_name: 'QQ 官方机器人',
        configured: Boolean(qqOfficial), enabled: false, state: 'idle', summary: '适配器未配置。',
      }] as never
      return { adapters: adaptersStore.adapters } as never
    })

    const wrapper = mount(QQOfficialPage, { attachTo: document.body, global: { plugins: [Antd, router] } })
    await flushPromises()
    return { wrapper, configStore, saveConfig }
  }

  it('shows the adapter status alongside its configuration', async () => {
    const { wrapper } = await mountPage()
    expect(wrapper.find('[data-testid="qq-adapter-status"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('适配器未配置。')
  })

  it('edits the qq_official block without disturbing the rest of the config', async () => {
    const { wrapper, configStore, saveConfig } = await mountPage({
      enabled: false, app_id: '', app_secret: '', intents: [], sandbox: false,
    })

    await wrapper.get('[data-testid="qq-app-id"]').setValue('102209770')
    await flushPromises()
    await wrapper.get('[data-testid="qq-save"]').trigger('click')
    await flushPromises()

    expect(saveConfig).toHaveBeenCalledTimes(1)
    const saved = saveConfig.mock.calls[0][0] as unknown as Record<string, Record<string, unknown>>
    expect(saved.qq_official.app_id).toBe('102209770')
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
