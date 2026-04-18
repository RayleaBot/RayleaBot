import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import ProtocolCompatibilityPage from '@/views/protocols/ProtocolCompatibilityView.vue'
import { useProtocolCompatibilityStore } from '@/stores/protocol-compatibility'
import { useProtocolsStore } from '@/stores/protocols'

function createProtocolSnapshot(overrides: Record<string, unknown> = {}) {
  return {
    protocol: 'onebot11',
    provider: 'napcat',
    configured_transports: ['forward_ws', 'webhook'],
    active_transports: ['forward_ws'],
    transport_status: [
      { transport: 'reverse_ws', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
      { transport: 'forward_ws', enabled: true, configured: true, endpoint: 'ws://127.0.0.1:8089', state: 'connected', summary: '主动连接已建立' },
      { transport: 'http_api', enabled: false, configured: false, endpoint: '', state: 'idle', summary: '未启用' },
      { transport: 'webhook', enabled: true, configured: true, endpoint: 'https://bot.example.com', state: 'listening', summary: 'Webhook 入口可接收上报' },
    ],
    readiness_status: 'ready',
    summary: 'OneBot11 主动连接已就绪',
    recent_transport_issues: [],
    ...overrides,
  }
}

function createCompatibilityMatrix() {
  return {
    protocol: 'onebot11',
    categories: [
      {
        key: 'events',
        title: '核心事件',
        items: [
          {
            key: 'message.group',
            label: '群消息',
            support: { standard: 'supported', napcat: 'supported', luckylillia: 'supported' },
            summary: '群消息事件进入正式插件事件主链。',
          },
        ],
      },
      {
        key: 'message_segments',
        title: '消息段',
        items: [
          {
            key: 'flash_file',
            label: '闪传文件',
            support: { standard: 'supported', napcat: 'supported', luckylillia: 'supported' },
            summary: '闪传文件消息段进入正式入站与出站消息段集合。',
          },
        ],
      },
      {
        key: 'read_capabilities',
        title: '读取能力',
        items: [
          {
            key: 'message.forward.get',
            label: '读取转发消息',
            support: { standard: 'supported', napcat: 'supported', luckylillia: 'supported' },
            summary: '平台提供转发消息详情读取能力。',
          },
        ],
      },
      {
        key: 'provider_extensions',
        title: 'Provider 扩展',
        items: [
          {
            key: 'provider.napcat.group.sign.set',
            label: 'NapCat 群签到',
            support: { standard: 'unsupported', napcat: 'supported', luckylillia: 'unsupported' },
            summary: 'NapCat 提供群签到扩展动作。',
          },
        ],
      },
    ],
  }
}

describe('ProtocolCompatibilityPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  function createTestRouter() {
    return createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/protocols/compatibility', component: ProtocolCompatibilityPage },
      ],
    })
  }

  it('renders four compatibility sections and highlights the current provider column', async () => {
    const protocolsStore = useProtocolsStore()
    const compatibilityStore = useProtocolCompatibilityStore()

    protocolsStore.snapshot = createProtocolSnapshot()
    compatibilityStore.matrix = createCompatibilityMatrix()

    vi.spyOn(protocolsStore, 'refresh').mockResolvedValue({ snapshot: protocolsStore.snapshot! })
    vi.spyOn(compatibilityStore, 'refresh').mockResolvedValue({ matrix: compatibilityStore.matrix! })

    const router = createTestRouter()
    await router.push('/protocols/compatibility')
    await router.isReady()

    const wrapper = mount(ProtocolCompatibilityPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('协议兼容矩阵')
    expect(wrapper.text()).toContain('NapCat')
    expect(wrapper.text()).toContain('主动连接 WebSocket')
    expect(wrapper.text()).toContain('Webhook')
    expect(wrapper.find('[data-testid="protocol-compatibility-events"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="protocol-compatibility-message_segments"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="protocol-compatibility-read_capabilities"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="protocol-compatibility-provider_extensions"]').exists()).toBe(true)
    expect(wrapper.findAll('th.is-current-provider').some((cell) => cell.text().includes('NapCat'))).toBe(true)
    expect(wrapper.text()).toContain('不支持')
    expect(wrapper.text()).toContain('provider.napcat.group.sign.set')
  })
})
