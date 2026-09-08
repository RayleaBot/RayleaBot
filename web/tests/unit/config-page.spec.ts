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
    const saveSpy = vi.spyOn(store, 'saveConfig').mockResolvedValue({
      config: store.document,
      redacted_fields: store.redactedFields,
      restart_required: true,
      apply_effects: {
        applied_now: ['log.level'],
        reloaded_now: [],
        restart_required_fields: ['server.port'],
      },
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

    const saveButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('保存更改'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')

    expect(saveSpy).toHaveBeenCalledTimes(1)
    expect(saveSpy.mock.calls[0][0].server.host).toBe('0.0.0.0')
  })

  it('keeps protocol fields out of the general config page', async () => {
    const store = useConfigStore()
    store.document = createConfigDocumentFixture()

    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)

    const wrapper = mount(ConfigPage, {
      global: {
        plugins: [getActivePinia()!],
      },
    })

    await flushPromises()

    expect(wrapper.find('.app-page').exists()).toBe(true)
    expect(wrapper.find('.config-page').exists()).toBe(true)
    expect(wrapper.find('.config-stack').exists()).toBe(true)
    expect(wrapper.find('.config-toc').exists()).toBe(true)
    expect(wrapper.find('.config-toolbar').exists()).toBe(true)
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

    const saveButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('保存更改'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')

    expect(saveSpy).toHaveBeenCalledTimes(1)
    expect(saveSpy.mock.calls[0][0].server.port).toBeUndefined()
  })

  it('edits image generation defaults from the render section', async () => {
    const store = useConfigStore()
    store.document = createConfigDocumentFixture()

    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)
    const saveSpy = vi.spyOn(store, 'saveConfig').mockResolvedValue({
      config: store.document,
      redacted_fields: [],
      restart_required: false,
      apply_effects: {
        applied_now: ['render.default_output', 'render.device_scale_percent'],
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

    const outputRow = getConfigFieldRow(wrapper, 'render.default_output')
    const precisionRow = getConfigFieldRow(wrapper, 'render.device_scale_percent')
    expect(precisionRow.props('field')).toMatchObject({ min: 50, max: 500, unit: '%' })

    await outputRow.vm.$emit('update:value', 'jpeg')
    await precisionRow.vm.$emit('update:value', 200)
    await flushPromises()

    const saveButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('保存更改'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')

    expect(saveSpy).toHaveBeenCalledTimes(1)
    const submitted = saveSpy.mock.calls[0][0]
    expect(submitted.render.default_output).toBe('jpeg')
    expect(submitted.render.device_scale_percent).toBe(200)
  })

  it('edits hot credential checks and restart-required Douyin browser settings', async () => {
    const store = useConfigStore()
    store.document = createConfigDocumentFixture()

    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)
    const saveSpy = vi.spyOn(store, 'saveConfig').mockResolvedValue({
      config: store.document,
      redacted_fields: [],
      restart_required: true,
      apply_effects: {
        applied_now: ['third_party_accounts.credential_check_interval_minutes'],
        reloaded_now: [],
        restart_required_fields: [
          'third_party_accounts.douyin_login.browser_mode',
          'third_party_accounts.douyin_login.remote_debugging_url',
        ],
      },
    })

    const wrapper = mount(ConfigPage, {
      global: {
        plugins: [getActivePinia()!],
      },
    })

    await flushPromises()

    const checkIntervalRow = getConfigFieldRow(wrapper, 'third_party_accounts.credential_check_interval_minutes')
    const browserModeRow = getConfigFieldRow(wrapper, 'third_party_accounts.douyin_login.browser_mode')
    const remoteDebuggingRow = getConfigFieldRow(wrapper, 'third_party_accounts.douyin_login.remote_debugging_url')
    expect(checkIntervalRow.props('field').restartRequired).toBeFalsy()
    expect(browserModeRow.props('field').restartRequired).toBe(true)
    expect(remoteDebuggingRow.props('field').restartRequired).toBe(true)

    await checkIntervalRow.vm.$emit('update:value', 720)
    await browserModeRow.vm.$emit('update:value', 'remote_cdp')
    await remoteDebuggingRow.vm.$emit('update:value', 'http://127.0.0.1:9222')
    await flushPromises()

    const saveButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('保存更改'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')

    expect(saveSpy).toHaveBeenCalledTimes(1)
    expect(saveSpy.mock.calls[0][0].third_party_accounts.douyin_login).toEqual({
      browser_mode: 'remote_cdp',
      remote_debugging_url: 'http://127.0.0.1:9222',
    })
    expect(saveSpy.mock.calls[0][0].third_party_accounts.credential_check_interval_minutes).toBe(720)
  })

  it('edits general IPC rate limit with split inputs', async () => {
    const store = useConfigStore()
    store.document = createConfigDocumentFixture()

    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)
    const saveSpy = vi.spyOn(store, 'saveConfig').mockResolvedValue({
      config: store.document,
      redacted_fields: [],
      restart_required: false,
      apply_effects: {
        applied_now: ['runtime.ipc_action_burst_limit'],
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

    await getConfigFieldRow(wrapper, 'runtime.ipc_action_burst_limit').vm.$emit('update:value', '200/10s')
    await flushPromises()

    const saveButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('保存更改'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')

    expect(saveSpy).toHaveBeenCalledTimes(1)
    const submitted = saveSpy.mock.calls[0][0]
    expect(submitted.runtime.ipc_action_burst_limit).toBe('200/10s')
    expect(submitted.message.rate_limit_per_target).toBe('5/5s')
  })

  it('reflects dirty state in the toolbar save button', async () => {
    const store = useConfigStore()
    store.document = createConfigDocumentFixture()
    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)

    const wrapper = mount(ConfigPage, {
      global: {
        plugins: [getActivePinia()!],
      },
    })

    await flushPromises()

    const saveButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('保存更改'))
    expect(saveButton).toBeTruthy()
    expect((saveButton!.element as HTMLButtonElement).disabled).toBe(true)

    await getConfigFieldRow(wrapper, 'server.host').vm.$emit('update:value', '0.0.0.0')
    await flushPromises()

    expect((saveButton!.element as HTMLButtonElement).disabled).toBe(false)
  })
})
