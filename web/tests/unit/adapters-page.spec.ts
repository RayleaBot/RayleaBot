import { defineComponent, watch } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { DOMWrapper, flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter, RouterView } from 'vue-router'

import AdaptersPage from '@/views/protocols/AdaptersView.vue'
import { buildAdapterInstance } from '@/lib/adapters'
import { useAdaptersStore } from '@/stores/adapters'
import { useConfigStore } from '@/stores/config'
import { useProtocolsStore } from '@/stores/protocols'
import type { ConfigDocument } from '@/types/api'
import { createConfigDocumentFixture } from './config-document.fixture'

vi.mock('@/adapter/feedback', () => ({ notifyError: vi.fn(), notifySuccess: vi.fn(), useToastFeedback: vi.fn() }))
const DialogStub = defineComponent({
  props: ['open', 'title', 'role', 'busy'],
  emits: ['close', 'afterClose'],
  setup(props, { emit }) {
    watch(() => props.open, (open) => { if (!open) emit('afterClose') })
  },
  template: '<section v-if="open" :role="role || \'dialog\'"><h2>{{ title }}</h2><button aria-label="关闭弹窗" :disabled="busy" @click="$emit(\'close\')">关闭</button><slot /><slot name="footer" /></section>',
})
const wrappers: VueWrapper[] = []
const body = () => new DOMWrapper(document.body)
const copy = <T,>(value: T): T => JSON.parse(JSON.stringify(value))

beforeEach(() => { setActivePinia(createPinia()) })
afterEach(() => {
  wrappers.splice(0).forEach((wrapper) => wrapper.unmount())
  document.body.innerHTML = ''
  vi.restoreAllMocks()
})

async function setup(path = '/protocols', document = createConfigDocumentFixture()) {
  let serverDocument = copy(document)
  const config = useConfigStore()
  const fetch = vi.spyOn(config, 'fetchConfig').mockImplementation(async () => { config.document = copy(serverDocument) })
  const save = vi.spyOn(config, 'saveConfig').mockImplementation(async (value) => {
    serverDocument = copy(value)
    config.document = copy(value)
    return { config: copy(value), redacted_fields: [], restart_required: true, apply_effects: { applied_now: [], reloaded_now: [], restart_required_fields: ['adapters'] } }
  })
  const adapters = useAdaptersStore()
  vi.spyOn(adapters, 'refresh').mockImplementation(async () => {
    adapters.adapters = serverDocument.adapters.map((entry) => ({
      id: entry.id, protocol: entry.type, display_name: entry.type === 'onebot11' ? 'OneBot11' : 'QQ 官方机器人', enabled: entry.enabled,
      state: 'stopped', summary: `状态来自 ${entry.id}`,
    }))
    adapters.availableProtocols = [
      { protocol: 'onebot11', display_name: 'OneBot11', description: 'OneBot 客户端' },
      { protocol: 'qqofficial', display_name: 'QQ 官方机器人', description: 'QQ 开放平台' },
    ]
    return { adapters: adapters.adapters, available_protocols: adapters.availableProtocols }
  })
  vi.spyOn(useProtocolsStore(), 'refresh').mockResolvedValue({ snapshot: null } as never)
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/protocols', name: 'protocols', component: AdaptersPage },
    { path: '/logs', name: 'logs', component: { template: '<div>logs</div>' } },
  ] })
  await router.push(path)
  await router.isReady()
  const wrapper = mount(RouterView, { attachTo: window.document.body, global: { plugins: [router], stubs: { AppDialog: DialogStub } } })
  wrappers.push(wrapper)
  await flushPromises()
  return { wrapper, router, save, fetch, config, adapters, updateServer: (update: (document: ConfigDocument) => void) => update(serverDocument) }
}

async function choose(protocol = 'qqofficial') {
  await body().get('[data-testid="adapter-add"]').trigger('click')
  await flushPromises()
  await body().get(`[data-testid="adapter-select-${protocol}"]`).trigger('click')
  await flushPromises()
}
async function saveForm() {
  await body().get('[data-testid="adapter-save"]').trigger('click')
  await flushPromises()
}

describe('protocol center dialogs', () => {
  it('shows configured connections and keeps protocol choices inside the add dialog', async () => {
    const { wrapper, router, save } = await setup()
    expect(wrapper.find('[data-testid="adapter-onebot11"]').exists()).toBe(true)
    expect(body().find('[data-testid="adapter-select-onebot11"]').exists()).toBe(false)
    await choose()
    expect(router.currentRoute.value.path).toBe('/protocols')
    expect(router.currentRoute.value.query.view).toBe('add')
    expect(save).not.toHaveBeenCalled()
    expect(body().get('#adapter-app-id').exists()).toBe(true)
  })

  it('validates before creation, then merges into the latest config and reports restart', async () => {
    const { save, router, updateServer } = await setup()
    await choose()
    await saveForm()
    expect(save).not.toHaveBeenCalled()
    expect(body().text()).toContain('请输入 AppSecret')
    await body().get('#adapter-app-id').setValue('100000001')
    await body().get('#adapter-app-secret').setValue('fixture-new-secret')
    updateServer((doc) => { doc.server.port = 8091; doc.adapters.push(buildAdapterInstance('another', 'onebot11')) })
    await saveForm()
    expect(save).toHaveBeenCalledTimes(1)
    const saved = save.mock.calls[0][0]
    expect(saved.server.port).toBe(8091)
    expect(saved.adapters.map((entry) => entry.id)).toEqual(['onebot11', 'another', 'qq-official'])
    expect(saved.adapters[2]).toMatchObject({ enabled: true, qqofficial: { app_id: '100000001', app_secret: 'fixture-new-secret', intents: ['group_and_c2c'] } })
    expect(router.currentRoute.value.query).toEqual({})
    expect(body().find('[data-testid="adapter-restart-notice"]').exists()).toBe(true)
  })

  it('protects drafts when closing or navigating, and cancellation never persists an instance', async () => {
    const { save, router } = await setup()
    await choose()
    await body().get('#adapter-app-id').setValue('123')
    await body().get('[aria-label="关闭弹窗"]').trigger('click')
    await flushPromises()
    expect(body().find('[role=alertdialog]').exists()).toBe(true)
    const clickConfirmation = async (text: string) => {
      await body().findAll('[role=alertdialog] button').find((button) => button.text() === text)!.trigger('click')
      await flushPromises()
    }
    await clickConfirmation('继续编辑')
    expect(body().get('#adapter-app-id').element).toHaveProperty('value', '123')
    const navigation = router.push('/logs')
    await flushPromises()
    await clickConfirmation('继续编辑')
    await navigation
    expect(router.currentRoute.value.path).toBe('/protocols')
    await body().get('[aria-label="关闭弹窗"]').trigger('click')
    await flushPromises()
    await clickConfirmation('放弃修改')
    expect(router.currentRoute.value.query).toEqual({})
    expect(save).not.toHaveBeenCalled()
    expect(body().find('#adapter-app-id').exists()).toBe(false)
  })

  it('edits one QQ connection while preserving its masked secret and other instances', async () => {
    const doc = createConfigDocumentFixture()
    const qq = buildAdapterInstance('qq-official', 'qqofficial')
    qq.qqofficial = { app_id: '100', app_secret: '********', intents: ['guilds', 'direct_message'], sandbox: true }
    doc.adapters.push(qq)
    const { save, updateServer } = await setup('/protocols?adapter=qq-official', doc)
    expect(body().get('[data-testid="adapter-save"]').attributes('disabled')).toBeDefined()
    expect(body().get('#adapter-id').attributes('disabled')).toBeDefined()
    await body().get('#adapter-app-id').setValue('200')
    updateServer((next) => { next.adapters[0].enabled = true })
    await saveForm()
    expect(save.mock.calls[0][0].adapters[0].enabled).toBe(true)
    expect(save.mock.calls[0][0].adapters[1].qqofficial).toEqual({ ...qq.qqofficial, app_id: '200' })
  })

  it('retains input on a same-instance conflict and does not overwrite the server', async () => {
    const doc = createConfigDocumentFixture()
    doc.adapters = [buildAdapterInstance('qq-official', 'qqofficial')]
    const { save, updateServer } = await setup('/protocols?adapter=qq-official', doc)
    await body().get('#adapter-app-id').setValue('200')
    updateServer((next) => { next.adapters[0].qqofficial!.app_id = '300' })
    await saveForm()
    expect(save).not.toHaveBeenCalled()
    expect(body().text()).toContain('已在其他位置更改或删除')
    expect(body().get('#adapter-app-id').element).toHaveProperty('value', '200')
  })

  it('keeps filled configuration after a rejected save and prevents duplicate writes', async () => {
    const { save, router } = await setup()
    await choose()
    await body().get('#adapter-app-id').setValue('100')
    await body().get('#adapter-app-secret').setValue('fixture-secret')
    let reject!: (reason: Error) => void
    save.mockImplementationOnce(() => new Promise((_, rejectSave) => { reject = rejectSave }))
    await saveForm()
    await saveForm()
    expect(save).toHaveBeenCalledTimes(1)
    await router.push('/logs')
    expect(router.currentRoute.value.path).toBe('/protocols')
    reject(new Error('fixture failure'))
    await flushPromises()
    expect(body().get('#adapter-app-id').element).toHaveProperty('value', '100')
    expect(body().get('[data-testid="adapter-save"]').attributes('disabled')).toBeUndefined()
    await saveForm()
    expect(save).toHaveBeenCalledTimes(2)
  })

  it('preserves multiple OneBot transports, custom callback address, and separate credentials', async () => {
    const doc = createConfigDocumentFixture()
    const settings = doc.adapters[0].onebot11!
    settings.reverse_ws = { enabled: true, url: 'wss://bot.example/api/adapters/onebot11/reverse-ws', access_token: '********', access_token_query_compat: true }
    settings.http_api = { enabled: true, url: 'https://client.example/api', access_token: '********' }
    settings.webhook = { enabled: true, url: 'https://bot.example/api/adapters/onebot11/webhook', access_token: 'fixture-webhook', access_token_query_compat: false }
    const { save } = await setup('/protocols?adapter=onebot11', doc)
    await body().get('#adapter-enabled').trigger('click')
    await saveForm()
    expect(save).toHaveBeenCalledTimes(1)
    expect(save.mock.calls[0][0].adapters[0].onebot11).toEqual(settings)
    expect(save.mock.calls[0][0].adapters[0].enabled).toBe(true)
  })

  it('does not show the primary OneBot runtime details for a different instance', async () => {
    const doc = createConfigDocumentFixture()
    doc.adapters.push(buildAdapterInstance('secondary', 'onebot11'))
    const { wrapper } = await setup('/protocols?adapter=secondary', doc)
    expect(wrapper.text()).toContain('状态来自 secondary')
    expect(body().find('.runtime-list').exists()).toBe(false)
    expect(useProtocolsStore().refresh).not.toHaveBeenCalled()
  })

  it('removes only the chosen connection after confirmation using a fresh document', async () => {
    const doc = createConfigDocumentFixture()
    doc.adapters.push(buildAdapterInstance('second', 'onebot11'))
    const { wrapper, save, updateServer } = await setup('/protocols', doc)
    updateServer((next) => { next.server.port = 9000 })
    await wrapper.get('[aria-label="删除连接 second"]').trigger('click')
    await flushPromises()
    await body().findAll('[role=alertdialog] button').find((button) => button.text() === '删除')!.trigger('click')
    await flushPromises()
    expect(save.mock.calls[0][0].adapters.map((entry) => entry.id)).toEqual(['onebot11'])
    expect(save.mock.calls[0][0].server.port).toBe(9000)
  })

  it('shows a loading failure without presenting it as an empty configuration', async () => {
    const { config, fetch, wrapper } = await setup()
    config.document = null
    fetch.mockImplementationOnce(async () => { config.error = '读取失败'; throw new Error('offline') })
    await wrapper.get('[aria-label="刷新连接状态"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('读取失败')
    expect(wrapper.text()).not.toContain('还没有机器人连接')
  })
})
