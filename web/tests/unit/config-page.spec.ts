import { createPinia, getActivePinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { notifySuccess } from '@/adapter/feedback'
import ConfigFieldRow from '@/components/config/ConfigFieldRow.vue'
import ConfigPage from '@/views/system/ConfigView.vue'
import { useConfigStore } from '@/stores/config'
import { createConfigDocumentFixture } from './config-document.fixture'

vi.mock('@/adapter/feedback', () => ({
  notifySuccess: vi.fn(),
  useToastFeedback: vi.fn(),
}))

function getConfigFieldRow(wrapper: ReturnType<typeof mount>, path: string) {
  const row = wrapper.findAllComponents(ConfigFieldRow).find(candidate => candidate.props('field').path === path)
  expect(row).toBeDefined()
  return row!
}

async function selectCategory(wrapper: ReturnType<typeof mount>, key: string, advanced = false) {
  await wrapper.get(`[data-category="${key}"]`).trigger('mousedown', { button: 0, ctrlKey: false })
  await flushPromises()
  if (advanced) {
    await wrapper.get('.config-advanced__trigger').trigger('click')
    await flushPromises()
  }
}

describe('ConfigPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(notifySuccess).mockClear()
  })

  it('submits the edited config document', async () => {
    const store = useConfigStore()
    store.document = createConfigDocumentFixture()
    store.redactedFields = []

    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)
    const saveSpy = vi.spyOn(store, 'saveConfig').mockImplementation(async () => {
      store.restartRequired = true
      return {
        config: store.document!,
        redacted_fields: store.redactedFields,
        restart_required: true,
        apply_effects: {
          applied_now: ['log.level'],
          reloaded_now: [],
          restart_required_fields: ['server.port'],
        },
      }
    })

    const wrapper = mount(ConfigPage, {
      global: {
        plugins: [getActivePinia()!],
      },
    })

    await flushPromises()
    expect(wrapper.text()).toContain('监听地址')
    expect(wrapper.text()).toContain('服务监听')
    const hostInput = wrapper.find('#config-field-server-host')
    expect(hostInput.exists()).toBe(true)
    await hostInput.setValue('0.0.0.0')
    await flushPromises()

    const saveButton = wrapper.get('[data-testid=config-save]')
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')

    expect(saveSpy).toHaveBeenCalledTimes(1)
    expect(saveSpy.mock.calls[0][0].server.host).toBe('0.0.0.0')
    await flushPromises()
    // A restart still owed after the save stays on the page, not only in the button's tooltip.
    expect(wrapper.get('[data-testid=config-restart-notice]').text()).toContain('需要重启服务')
  })

  it('keeps cleared numeric fields empty instead of forcing them to 0', async () => {
    const store = useConfigStore()
    store.document = createConfigDocumentFixture()

    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)
    const saveSpy = vi.spyOn(store, 'saveConfig').mockResolvedValue({
      config: store.document,
      redacted_fields: [],
      restart_required: false,
      apply_effects: {
        applied_now: ['command.prefixes'],
        reloaded_now: [],
        restart_required_fields: [],
      },
    })

    const wrapper = mount(ConfigPage, {
      global: {
        plugins: [getActivePinia()!],
      },
    })

    await flushPromises()

    const portInput = wrapper.find('input.config-field__number')
    expect(portInput.exists()).toBe(true)
    await portInput.setValue('')

    const saveButton = wrapper.get('[data-testid=config-save]')
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')

    expect(saveSpy).toHaveBeenCalledTimes(1)
    expect(saveSpy.mock.calls[0][0].server.port).toBeUndefined()
  })

  it('keeps edits across category changes and searches including advanced fields', async () => {
    const store = useConfigStore()
    store.document = createConfigDocumentFixture()
    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)
    const wrapper = mount(ConfigPage, { global: { plugins: [getActivePinia()!] } })
    await flushPromises()
    await wrapper.get('#config-field-server-host').setValue('0.0.0.0')
    await selectCategory(wrapper, 'render')
    expect(wrapper.find('#config-field-render-browser_path').exists()).toBe(false)
    await wrapper.get('#config-field-render-device_scale_percent').setValue('200')
    await wrapper.get('[aria-label="搜索配置项"]').setValue('browser_args')
    await flushPromises()
    expect(getConfigFieldRow(wrapper, 'render.browser_args').exists()).toBe(true)
    await wrapper.get('[aria-label="搜索配置项"]').setValue('不存在的配置')
    await flushPromises()
    expect(wrapper.text()).toContain('没有找到配置项')
    const clear = wrapper.findAll('button').find(button => button.text() === '清除搜索')!
    await clear.trigger('click')
    await flushPromises()
    expect((wrapper.get('#config-field-render-device_scale_percent').element as HTMLInputElement).value).toBe('200')
    await selectCategory(wrapper, 'access')
    expect((wrapper.get('#config-field-server-host').element as HTMLInputElement).value).toBe('0.0.0.0')
    expect(wrapper.get('#config-save-status').text()).toContain('2 项未保存')
  })

  it('retains the local draft when a newer snapshot arrives and discards to that snapshot', async () => {
    const store = useConfigStore()
    store.document = createConfigDocumentFixture()
    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)
    const wrapper = mount(ConfigPage, { global: { plugins: [getActivePinia()!] } })
    await flushPromises()
    await wrapper.get('#config-field-server-port').setValue('22334')
    store.document = { ...createConfigDocumentFixture(), server: { ...store.document.server, port: 22335 } }
    await flushPromises()
    expect((wrapper.get('#config-field-server-port').element as HTMLInputElement).value).toBe('22334')
    expect(wrapper.text()).toContain('已在其他位置更新')
    await wrapper.get('[aria-label=撤销修改]').trigger('click')
    await flushPromises()
    expect((wrapper.get('#config-field-server-port').element as HTMLInputElement).value).toBe('22335')
    expect(wrapper.get('#config-save-status').text()).toContain('无需保存')
  })

  it('keeps a failed save editable and restores the formal response after retry', async () => {
    const store = useConfigStore()
    store.document = createConfigDocumentFixture()
    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)
    const saveSpy = vi.spyOn(store, 'saveConfig').mockRejectedValueOnce(new Error('fixture failure'))
    const wrapper = mount(ConfigPage, { global: { plugins: [getActivePinia()!] } })
    await flushPromises()
    await wrapper.get('#config-field-server-host').setValue('0.0.0.0')
    const save = wrapper.get('[data-testid=config-save]')
    await save.trigger('click')
    await flushPromises()
    expect(wrapper.find('[role=alert]').exists()).toBe(true)
    expect((wrapper.get('#config-field-server-host').element as HTMLInputElement).value).toBe('0.0.0.0')
    expect(save.attributes('aria-disabled') === 'true').toBe(false)
    const formal = createConfigDocumentFixture()
    formal.server.host = '0.0.0.0'
    saveSpy.mockResolvedValueOnce({ config: formal, redacted_fields: [], restart_required: true, apply_effects: { applied_now: [], reloaded_now: [], restart_required_fields: ['server.host'] } })
    await save.trigger('click')
    await flushPromises()
    expect(wrapper.find('[role=alert]').exists()).toBe(false)
    expect(save.attributes('aria-disabled') === 'true').toBe(true)
  })
})
