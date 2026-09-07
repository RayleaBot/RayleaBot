import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { notifyError, notifySuccess, useToastFeedback } from '@/adapter/feedback'
import ProtocolsPage from '@/views/protocols/ProtocolsView.vue'
import { useConfigStore } from '@/stores/config'
import { useProtocolsStore } from '@/stores/protocols'
import { createConfigDocumentFixture } from './config-document.fixture'

vi.mock('@/adapter/feedback', () => ({
  notifyError: vi.fn(),
  notifySuccess: vi.fn(),
  useToastFeedback: vi.fn(),
}))

describe('ProtocolsPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    document.body.innerHTML = ''
    vi.mocked(notifySuccess).mockClear()
    vi.mocked(notifyError).mockClear()
    vi.mocked(useToastFeedback).mockClear()
  })

  // The page edits one OneBot instance, named by the route.
  const ADAPTER_ID = 'onebot11'
  const ADAPTER_ROUTE = `/protocols/onebot11/${ADAPTER_ID}`

  // savedOneBot reads the edited instance out of a saved config document.
  function savedOneBot(saved: Record<string, unknown>) {
    const instances = saved.adapters as Array<Record<string, unknown>>
    return instances.find((instance) => instance.id === ADAPTER_ID)!.onebot11 as Record<string, Record<string, unknown>>
  }

  function createTestRouter() {
    return createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/protocols/onebot11/:adapterId', name: 'protocols', component: ProtocolsPage },
        { path: '/protocols/compatibility', name: 'protocols-compatibility', component: { template: '<div>compatibility</div>' } },
        { path: '/logs', name: 'logs', component: { template: '<div>logs</div>' } },
      ],
    })
  }

  it('renders protocol settings with a readable chinese status summary', async () => {
    const configStore = useConfigStore()
    const protocolsStore = useProtocolsStore()

    configStore.document = createConfigDocumentFixture()
    configStore.redactedFields = []
    protocolsStore.snapshot = {
      protocol: 'onebot11',
      configured_transports: ['forward_ws'],
      active_transports: ['forward_ws'],
      transport_status: [
        { transport: 'reverse_ws', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
        {
          transport: 'forward_ws',
          enabled: true,
          configured: true,
          endpoint: 'ws://127.0.0.1:8089',
          state: 'auth_failed',
          summary: '主动连接鉴权失败',
          provider: 'napcat',
          app_name: 'NapCat.Onebot',
          protocol_version: 'v11',
          app_version: '1.0.0',
          user_id: '10001',
          nickname: 'TestBot',
        },
        { transport: 'http_api', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
        { transport: 'webhook', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
      ],
      readiness_status: 'degraded',
      summary: 'OneBot11 鉴权失败，请检查访问令牌',
      recent_transport_issues: [
        {
          code: 'adapter.transport_forward_ws_connection_failed',
          severity: 'warning',
          summary: 'OneBot 主动连接鉴权失败，请检查访问令牌。',
        },
      ],
    }

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(protocolsStore, 'refresh').mockResolvedValue({ snapshot: protocolsStore.snapshot! })

    const router = createTestRouter()
    await router.push(ADAPTER_ROUTE)
    await router.isReady()

    const wrapper = mount(ProtocolsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('协议中心')
    expect(wrapper.text()).toContain('OneBot11')
    expect(wrapper.text()).toContain('OneBot11 鉴权失败，请检查访问令牌')
    expect(wrapper.text()).toContain('传输状态')
    expect(wrapper.text()).toContain('连接端')
    expect(wrapper.text()).toContain('NapCat')
    expect(wrapper.text()).toContain('NapCat.Onebot')
    expect(wrapper.text()).toContain('v11 / 1.0.0')
    expect(wrapper.text()).toContain('登录账号')
    expect(wrapper.text()).toContain('10001 / TestBot')
    expect(wrapper.text()).toContain('传输异常')
    expect(wrapper.text()).toContain('主动连接 WebSocket')
    expect(wrapper.text()).toContain('主动连接鉴权失败')
    expect(wrapper.text()).toContain('adapter.transport_forward_ws_connection_failed')
    expect(wrapper.text()).toContain('OneBot 主动连接鉴权失败，请检查访问令牌。')
    expect(wrapper.text()).toContain('连接设置')
    expect(wrapper.findAll('input[aria-label="访问令牌"]')).toHaveLength(4)
    expect(document.body.querySelector('.ant-drawer')).toBeNull()
    expect(wrapper.find('[data-testid="protocol-unsaved-status"]').exists()).toBe(false)
    await wrapper.get('input[aria-label="主动连接地址"]').setValue('ws://127.0.0.1:8089')
    await flushPromises()
    expect(wrapper.get('[data-testid="protocol-unsaved-status"]').text()).toContain('协议设置尚未保存')
    expect(wrapper.html()).not.toContain('__REDACTED__')
    expect(wrapper.text()).toContain('兼容矩阵')
    expect(wrapper.text()).toContain('查看实时日志')

    const realtimeLogsButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('查看实时日志'))
    expect(realtimeLogsButton).toBeTruthy()
    await realtimeLogsButton!.trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.name).toBe('logs')
    expect(router.currentRoute.value.query.protocol).toBe('onebot11')
  })

  it('hides the transport issue section after the issue list is cleared', async () => {
    const configStore = useConfigStore()
    const protocolsStore = useProtocolsStore()

    configStore.document = createConfigDocumentFixture()
    protocolsStore.snapshot = {
      protocol: 'onebot11',
      configured_transports: ['forward_ws'],
      active_transports: ['forward_ws'],
      transport_status: [
        { transport: 'reverse_ws', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
        { transport: 'forward_ws', enabled: true, configured: true, endpoint: 'ws://127.0.0.1:8089', state: 'reconnecting', summary: '连接已断开，正在重试' },
        { transport: 'http_api', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
        { transport: 'webhook', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
      ],
      readiness_status: 'degraded',
      summary: 'OneBot11 传输链路部分可用',
      recent_transport_issues: [
        {
          code: 'adapter.transport_forward_ws_session_lost',
          severity: 'warning',
          summary: 'OneBot 主动连接已断开，正在重试。',
        },
      ],
    }

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(protocolsStore, 'refresh').mockResolvedValue({ snapshot: protocolsStore.snapshot! })

    const router = createTestRouter()
    await router.push(ADAPTER_ROUTE)
    await router.isReady()

    const wrapper = mount(ProtocolsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.find('[data-testid="protocol-issues"]').exists()).toBe(true)

    protocolsStore.snapshot = {
      ...protocolsStore.snapshot!,
      readiness_status: 'ready',
      recent_transport_issues: [],
    }
    await flushPromises()

    expect(wrapper.find('[data-testid="protocol-issues"]').exists()).toBe(false)
  })

  it('submits the full config document while editing protocol fields only', async () => {
    const configStore = useConfigStore()
    const protocolsStore = useProtocolsStore()

    configStore.document = createConfigDocumentFixture()
    protocolsStore.snapshot = {
      protocol: 'onebot11',
      configured_transports: ['forward_ws'],
      active_transports: ['forward_ws'],
      transport_status: [
        { transport: 'reverse_ws', enabled: true, configured: true, endpoint: 'wss://bot.example.com/reverse', state: 'listening', summary: '等待 OneBot 回连' },
        { transport: 'forward_ws', enabled: true, configured: true, endpoint: 'ws://127.0.0.1:8089', state: 'connected', summary: '主动连接已建立' },
        { transport: 'http_api', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
        { transport: 'webhook', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
      ],
      readiness_status: 'ready',
      summary: 'OneBot11 主动连接已就绪',
      recent_transport_issues: [],
    }

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    const refreshSpy = vi.spyOn(protocolsStore, 'refresh')
      .mockResolvedValueOnce({ snapshot: protocolsStore.snapshot! })
      .mockImplementationOnce(async () => {
        protocolsStore.snapshot = {
          protocol: 'onebot11',
          configured_transports: ['reverse_ws', 'forward_ws'],
          active_transports: ['forward_ws'],
          transport_status: [
            { transport: 'reverse_ws', enabled: false, configured: true, endpoint: 'wss://bot.example.com/reverse/onebot', state: 'idle', summary: '未启用' },
            { transport: 'forward_ws', enabled: true, configured: true, endpoint: 'ws://127.0.0.1:8089', state: 'connected', summary: '主动连接已建立' },
            { transport: 'http_api', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
            { transport: 'webhook', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
          ],
          readiness_status: 'degraded',
          summary: 'OneBot11 等待回连',
          recent_transport_issues: [],
        }
        return { snapshot: protocolsStore.snapshot! }
      })
    const saveSpy = vi.spyOn(configStore, 'saveConfig').mockResolvedValue({
      config: configStore.document,
      redacted_fields: [],
      restart_required: false,
      apply_effects: {
        applied_now: [],
        reloaded_now: ['adapters.onebot11.onebot11.reverse_ws.url'],
        restart_required_fields: [],
      },
    })

    const router = createTestRouter()
    await router.push(ADAPTER_ROUTE)
    await router.isReady()

    const wrapper = mount(ProtocolsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    const reverseTransportRow = wrapper.findAll('.ant-table-row')
      .find((candidate) => candidate.text().includes('回连 WebSocket'))
    expect(reverseTransportRow).toBeTruthy()

    const wsUrlInput = reverseTransportRow!.get<HTMLInputElement>('input[aria-label="协议端回连地址"]')
    expect(wsUrlInput.attributes('readonly')).toBeDefined()
    expect(wsUrlInput.element.value).toBe('ws://127.0.0.1:8080/api/adapters/onebot11/reverse-ws')
    const reverseSwitch = reverseTransportRow!.get('[role="switch"]')
    await reverseSwitch.trigger('click')
    const tokenInput = reverseTransportRow!.get<HTMLInputElement>('input[aria-label="访问令牌"]')
    await tokenInput.setValue('reverse-secret')
    await flushPromises()

    expect(wrapper.get('[data-testid="protocol-unsaved-status"]').text()).toContain('协议设置尚未保存')

    const saveButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('保存协议设置'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')

    expect(saveSpy).toHaveBeenCalledTimes(1)
    expect(refreshSpy).toHaveBeenCalledTimes(2)
    const savedSettings = savedOneBot(saveSpy.mock.calls[0][0])
    expect(savedSettings.reverse_ws.url).toBe('ws://127.0.0.1:8080/api/adapters/onebot11/reverse-ws')
    expect(savedSettings.reverse_ws.access_token).toBe('reverse-secret')
    expect(savedSettings.forward_ws.access_token).toBe('')
    expect('access_token' in savedSettings).toBe(false)
    expect(saveSpy.mock.calls[0][0].server.host).toBe('127.0.0.1')
    expect(wrapper.text()).toContain('未启用')
  })

  it('keeps cleared protocol numeric fields empty instead of forcing them to 0', async () => {
    const configStore = useConfigStore()
    const protocolsStore = useProtocolsStore()

    configStore.document = createConfigDocumentFixture()
    protocolsStore.snapshot = {
      protocol: 'onebot11',
      configured_transports: ['forward_ws'],
      active_transports: ['forward_ws'],
      transport_status: [
        { transport: 'reverse_ws', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
        { transport: 'forward_ws', enabled: true, configured: true, endpoint: 'ws://127.0.0.1:8089', state: 'connected', summary: '主动连接已建立' },
        { transport: 'http_api', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
        { transport: 'webhook', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
      ],
      readiness_status: 'ready',
      summary: 'OneBot11 主动连接已就绪',
      recent_transport_issues: [],
    }

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(protocolsStore, 'refresh').mockResolvedValue({ snapshot: protocolsStore.snapshot! })
    const saveSpy = vi.spyOn(configStore, 'saveConfig').mockResolvedValue({
      config: configStore.document,
      redacted_fields: [],
      restart_required: false,
      apply_effects: {
        applied_now: [],
        reloaded_now: ['adapter.connect_timeout_seconds'],
        restart_required_fields: [],
      },
    })

    const router = createTestRouter()
    await router.push(ADAPTER_ROUTE)
    await router.isReady()

    const wrapper = mount(ProtocolsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    const timeoutInput = wrapper.find('input[aria-label="连接超时（秒）"]')
    expect(timeoutInput.exists()).toBe(true)
    await timeoutInput.setValue('')

    const saveButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('保存协议设置'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')

    expect(saveSpy).toHaveBeenCalledTimes(1)
    expect(saveSpy.mock.calls[0][0].adapter.connect_timeout_seconds).toBeUndefined()
  })

  it('keeps apply effect details out of the protocol page banner area', async () => {
    const configStore = useConfigStore()
    const protocolsStore = useProtocolsStore()

    configStore.document = createConfigDocumentFixture()
    configStore.applyEffects = {
      applied_now: [],
      reloaded_now: ['adapters.onebot11.onebot11.forward_ws.url'],
      restart_required_fields: ['render.browser_args'],
    }
    configStore.restartRequired = true
    protocolsStore.snapshot = {
      protocol: 'onebot11',
      configured_transports: ['forward_ws'],
      active_transports: ['forward_ws'],
      transport_status: [
        { transport: 'reverse_ws', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
        { transport: 'forward_ws', enabled: true, configured: true, endpoint: 'ws://127.0.0.1:8089', state: 'connected', summary: '主动连接已建立' },
        { transport: 'http_api', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
        { transport: 'webhook', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
      ],
      readiness_status: 'ready',
      summary: 'OneBot11 主动连接已就绪',
      recent_transport_issues: [],
    }

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(protocolsStore, 'refresh').mockResolvedValue({ snapshot: protocolsStore.snapshot! })

    const router = createTestRouter()
    await router.push(ADAPTER_ROUTE)
    await router.isReady()

    const wrapper = mount(ProtocolsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).not.toContain('保存结果')
    expect(wrapper.text()).not.toContain('已重载')
    expect(wrapper.text()).not.toContain('需重启生效')
    expect(wrapper.text()).not.toContain('adapters.onebot11.onebot11.forward_ws.url')
    expect(wrapper.text()).not.toContain('render.browser_args')
    expect(vi.mocked(useToastFeedback)).toHaveBeenCalled()
  })

  it('shows unknown runtime fallback and omits provider from saved config', async () => {
    const configStore = useConfigStore()
    const protocolsStore = useProtocolsStore()

    configStore.document = createConfigDocumentFixture()
    protocolsStore.snapshot = {
      protocol: 'onebot11',
      provider: 'unknown',
      configured_transports: ['webhook'],
      active_transports: ['webhook'],
      transport_status: [
        { transport: 'reverse_ws', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
        { transport: 'forward_ws', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
        { transport: 'http_api', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
        { transport: 'webhook', enabled: true, configured: true, endpoint: 'https://bot.example.com', state: 'listening', summary: 'Webhook 入口可接收上报' },
      ],
      readiness_status: 'degraded',
      summary: 'OneBot11 仅 Webhook 上报可用',
      recent_transport_issues: [],
    }

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(protocolsStore, 'refresh').mockResolvedValue({ snapshot: protocolsStore.snapshot! })
    const saveSpy = vi.spyOn(configStore, 'saveConfig').mockResolvedValue({
      config: configStore.document,
      redacted_fields: [],
      restart_required: false,
      apply_effects: {
        applied_now: [],
        reloaded_now: [],
        restart_required_fields: [],
      },
    })

    const router = createTestRouter()
    await router.push(ADAPTER_ROUTE)
    await router.isReady()

    const wrapper = mount(ProtocolsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Webhook 入口可接收上报')
    expect(wrapper.text()).toContain('未知')
    expect(wrapper.text()).not.toContain('Provider')
    expect(wrapper.find('input[aria-label="Provider"]').exists()).toBe(false)

    await wrapper.get('input[aria-label="上报回调地址"]').setValue('https://bot.example.com/events')
    await flushPromises()

    const saveButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('保存协议设置'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')

    expect(saveSpy).toHaveBeenCalledTimes(1)
    expect('provider' in savedOneBot(saveSpy.mock.calls[0][0])).toBe(false)
  })
})
