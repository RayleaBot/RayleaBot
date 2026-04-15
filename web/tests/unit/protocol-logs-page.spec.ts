import { defineComponent, ref } from 'vue'
import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { formatDateTime } from '@/lib/format'
import VirtualDataViewport from '@/components/VirtualDataViewport.vue'
import ProtocolLogsPage from '@/views/protocols/ProtocolLogsView.vue'
import { useProtocolLogsStore } from '@/stores/protocol-logs'

describe('ProtocolLogsPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  function createTestRouter() {
    return createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/protocols', component: { template: '<div>协议中心</div>' } },
        { path: '/protocols/logs', component: ProtocolLogsPage },
      ],
    })
  }

  it('renders the protocol terminal stream and structured detail panel', async () => {
    const logsStore = useProtocolLogsStore()

    logsStore.items = [
      {
        log_id: 'log_adapter_live_0001',
        timestamp: '2026-04-08T10:16:00Z',
        level: 'warn',
        protocol: 'onebot11',
        source: 'adapter.onebot11',
        request_id: 'req_adapter_ignored_0001',
        message: 'ignored OneBot API response with unsupported echo',
      },
    ]
    logsStore.selectedLogId = 'log_adapter_live_0001'
    logsStore.currentDetail = {
      log_id: 'log_adapter_live_0001',
      timestamp: '2026-04-08T10:16:00Z',
      level: 'warn',
      protocol: 'onebot11',
      source: 'adapter.onebot11',
      request_id: 'req_adapter_ignored_0001',
      message: 'ignored OneBot API response with unsupported echo',
      details: {
        direction: 'inbound',
        frame_type: 'api.response.ignored',
        reason: 'api response echo must be a non-empty string',
        echo_value_type: 'number',
        payload_preview: {
          echo: 123,
          status: 'ok',
        },
      },
    }

    vi.spyOn(logsStore, 'fetchList').mockResolvedValue(logsStore.items)

    const router = createTestRouter()
    await router.push('/protocols/logs')
    await router.isReady()

    const wrapper = mount(ProtocolLogsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.find('.protocol-logs-workspace').exists()).toBe(true)
    expect(wrapper.find('.logs-sidebar .sidebar-card').exists()).toBe(true)
    expect(wrapper.text()).toContain('协议日志')
    expect(wrapper.find('.terminal-card').exists()).toBe(true)
    expect(wrapper.findAll('.terminal-line')).toHaveLength(1)
    expect(wrapper.findComponent(VirtualDataViewport).props('dynamicItemHeight')).toBe(true)
    expect(wrapper.text()).toContain('ignored OneBot API response with unsupported echo')
    expect(wrapper.text()).toContain('消息详情')
    expect(wrapper.text()).toContain('api.response.ignored')
    expect(wrapper.find('.json-content').text()).toContain('"echo": 123')
  })

  it('renders OneBot message detail fields with clear labels', async () => {
    const logsStore = useProtocolLogsStore()

    logsStore.items = [
      {
        log_id: 'log_bridge_0002',
        timestamp: '2026-04-09T10:27:22Z',
        level: 'info',
        protocol: 'onebot11',
        source: 'bridge',
        request_id: 'req_bridge_0002',
        message: '721011692: [测试群(860105388)]管理员/Alice(3001): 您好',
      },
    ]
    logsStore.selectedLogId = 'log_bridge_0002'
    logsStore.currentDetail = {
      log_id: 'log_bridge_0002',
      timestamp: '2026-04-09T10:27:22Z',
      level: 'info',
      protocol: 'onebot11',
      source: 'bridge',
      request_id: 'req_bridge_0002',
      message: '721011692: [测试群(860105388)]管理员/Alice(3001): 您好',
      details: {
        direction: 'inbound',
        event_kind: 'onebot11.message',
        event_type: 'message.group',
        post_type: 'message',
        message_type: 'group',
        event_timestamp: 1729679125,
        self_id: '721011692',
        conversation_type: 'group',
        conversation_id: '860105388',
        group_name: '测试群',
        message_id: '966671988',
        raw_message: '您好',
        message_format: 'array',
        font: 14,
        plain_text: '您好',
        sender: {
          user_id: '3001',
          nickname: 'Alice',
          card: '管理员',
          role: 'owner',
        },
      },
    }

    vi.spyOn(logsStore, 'fetchList').mockResolvedValue(logsStore.items)

    const router = createTestRouter()
    await router.push('/protocols/logs')
    await router.isReady()

    const wrapper = mount(ProtocolLogsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('会话 ID（群号或私聊对象）')
    expect(wrapper.text()).toContain('机器人 QQ 号')
    expect(wrapper.text()).toContain('群名称')
    expect(wrapper.text()).toContain('发送者昵称')
    expect(wrapper.text()).toContain('发送者 ID')
    expect(wrapper.text()).toContain('消息 ID')
    expect(wrapper.text()).toContain('您好')
    expect(wrapper.find('.line-source').text()).toBe('bridge · onebot11')
    expect(wrapper.findAll('.field-label').some((node) => node.text() === '群号')).toBe(false)
    const detailJson = wrapper.find('.json-content').text()
    expect(detailJson).toContain('"self_id": "721011692"')
    expect(detailJson).toContain('"group_name": "测试群"')
    expect(detailJson).toContain('"sender"')
    expect(detailJson).not.toContain('sender_id')
    expect(detailJson).not.toContain('sender_nickname')
    expect(detailJson).not.toContain('"group_id"')
    expect(detailJson).not.toContain('"time"')
    expect(detailJson).not.toContain('"message_seq"')
  })

  it('escapes directional control characters in protocol detail text and structured JSON', async () => {
    const logsStore = useProtocolLogsStore()
    const unsafeCard = '群星怒\u2066，大明云玩家\u202e~喵\u2069'
    const unsafeMessage = `721011692: [760384342]${unsafeCard}/没错，是魔法！(2896109796): 除了战猎这种抓不到加费就完全没法打的角色`

    logsStore.items = [
      {
        log_id: 'log_bridge_unsafe_0001',
        timestamp: '2026-04-14T02:49:45Z',
        level: 'info',
        protocol: 'onebot11',
        source: 'bridge',
        request_id: 'req_bridge_unsafe_0001',
        message: unsafeMessage,
      },
    ]
    logsStore.selectedLogId = 'log_bridge_unsafe_0001'
    logsStore.currentDetail = {
      log_id: 'log_bridge_unsafe_0001',
      timestamp: '2026-04-14T02:49:45Z',
      level: 'info',
      protocol: 'onebot11',
      source: 'bridge',
      request_id: 'req_bridge_unsafe_0001',
      message: unsafeMessage,
      details: {
        direction: 'inbound',
        conversation_id: '760384342',
        conversation_type: 'group',
        sender: {
          user_id: '2896109796',
          nickname: '没错，是魔法！',
          card: unsafeCard,
          role: 'member',
        },
        plain_text: '除了战猎这种抓不到加费就完全没法打的角色',
      },
    }

    vi.spyOn(logsStore, 'fetchList').mockResolvedValue(logsStore.items)

    const router = createTestRouter()
    await router.push('/protocols/logs')
    await router.isReady()

    const wrapper = mount(ProtocolLogsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    const senderCardField = wrapper.findAll('.detail-field-box').find((node) => (
      node.find('.field-label').text() === '发送者名片'
    ))

    expect(senderCardField).toBeTruthy()
    expect(wrapper.find('.line-text').text()).toContain('\\u2066')
    expect(wrapper.find('.detail-hero-message').text()).toContain('\\u2066')
    expect(senderCardField!.find('.field-value').text()).toContain('\\u2066')
    expect(senderCardField!.find('.field-value').text()).not.toContain('\u2066')

    const detailJson = wrapper.find('.json-content').text()
    expect(detailJson).toContain('"card": "群星怒\\u2066，大明云玩家\\u202e~喵\\u2069"')
    expect(detailJson).not.toContain('\u2066')
    expect(detailJson).not.toContain('\u202e')
    expect(detailJson).not.toContain('\u2069')
  })

  it('renders outbound delivery detail fields with clear labels', async () => {
    const logsStore = useProtocolLogsStore()

    logsStore.items = [
      {
        log_id: 'log_outbound_failed_0001',
        timestamp: '2026-04-10T09:18:01Z',
        level: 'warn',
        protocol: 'onebot11',
        source: 'adapter.onebot11',
        plugin_id: 'weather',
        request_id: 'req_runtime_delivery_0002',
        message: 'plugin weather command echo failed to deliver group message: hello world',
      },
    ]
    logsStore.selectedLogId = 'log_outbound_failed_0001'
    logsStore.currentDetail = {
      log_id: 'log_outbound_failed_0001',
      timestamp: '2026-04-10T09:18:01Z',
      level: 'warn',
      protocol: 'onebot11',
      source: 'adapter.onebot11',
      plugin_id: 'weather',
      request_id: 'req_runtime_delivery_0002',
      message: 'plugin weather command echo failed to deliver group message: hello world',
      details: {
        direction: 'outbound',
        action_kind: 'message.reply',
        delivery_kind: 'message.send',
        command_name: 'echo',
        target_type: 'group',
        target_id: '2001',
        plain_text: 'hello world',
        error_code: 'adapter.send_failed',
        reason: 'send rejected by upstream',
      },
    }

    vi.spyOn(logsStore, 'fetchList').mockResolvedValue(logsStore.items)

    const router = createTestRouter()
    await router.push('/protocols/logs')
    await router.isReady()

    const wrapper = mount(ProtocolLogsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('请求动作')
    expect(wrapper.text()).toContain('实际发送方式')
    expect(wrapper.text()).toContain('触发指令')
    expect(wrapper.text()).toContain('错误代码')
    expect(wrapper.text()).toContain('message.reply')
    expect(wrapper.text()).toContain('message.send')
    expect(wrapper.text()).toContain('weather')
    expect(wrapper.text()).toContain('echo')
    expect(wrapper.text()).toContain('adapter.send_failed')
  })

  it('loads log detail when a terminal line is selected', async () => {
    const logsStore = useProtocolLogsStore()

    logsStore.items = [
      {
        log_id: 'log_bridge_0001',
        timestamp: '2026-03-20T10:00:02Z',
        level: 'info',
        protocol: 'onebot11',
        source: 'bridge',
        request_id: 'req_bridge_0001',
        message: '721011692: [测试群(2001)]管理员/Alice(3001): hello bridge',
      },
    ]

    vi.spyOn(logsStore, 'fetchList').mockResolvedValue(logsStore.items)
    const selectSpy = vi.spyOn(logsStore, 'selectLog').mockResolvedValue(null)

    const router = createTestRouter()
    await router.push('/protocols/logs')
    await router.isReady()

    const wrapper = mount(ProtocolLogsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    await wrapper.find('.terminal-line').trigger('click')
    expect(selectSpy).toHaveBeenCalledWith('log_bridge_0001')
  })

  it('shows a pending-new badge while browsing history pages', async () => {
    const logsStore = useProtocolLogsStore()

    logsStore.items = [
      {
        log_id: 'log_adapter_live_0001',
        timestamp: '2026-04-08T10:16:00Z',
        level: 'warn',
        protocol: 'onebot11',
        source: 'adapter.onebot11',
        request_id: 'req_adapter_ignored_0001',
        message: 'ignored OneBot API response with unsupported echo',
      },
    ]
    logsStore.selectedLogId = 'log_adapter_live_0001'

    vi.spyOn(logsStore, 'fetchList').mockResolvedValue(logsStore.items)

    const router = createTestRouter()
    await router.push('/protocols/logs')
    await router.isReady()

    const wrapper = mount(ProtocolLogsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()
    logsStore.isLatestPage = false
    logsStore.pendingNewCount = 2

    await flushPromises()

    expect(wrapper.text()).toContain('有 2 条新日志')
    expect(wrapper.text()).toContain('回到最新')
  })

  it('formats scientific-notation timestamps in the protocol terminal stream', async () => {
    const logsStore = useProtocolLogsStore()

    logsStore.items = [
      {
        log_id: 'log_bridge_scientific_0001',
        timestamp: '1.775762955e+09',
        level: 'info',
        protocol: 'onebot11',
        source: 'bridge',
        request_id: 'req_bridge_scientific_0001',
        message: '10001: 乔温迪乔斯达(3599026669): 6',
      },
    ]
    logsStore.selectedLogId = 'log_bridge_scientific_0001'

    vi.spyOn(logsStore, 'fetchList').mockResolvedValue(logsStore.items)

    const router = createTestRouter()
    await router.push('/protocols/logs')
    await router.isReady()

    const wrapper = mount(ProtocolLogsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.find('.line-time').text()).toBe(formatDateTime('1.775762955e+09'))
    expect(wrapper.find('.line-source').text()).toBe('bridge · onebot11')
  })

  it('keeps the protocol log layout visible when a live log carries invalid time fields', async () => {
    const logsStore = useProtocolLogsStore()

    logsStore.items = [
      {
        log_id: 'log_bridge_0003',
        timestamp: '2026-04-09T10:27:21Z',
        level: 'info',
        protocol: 'onebot11',
        source: 'bridge',
        request_id: 'req_bridge_0003',
        message: '721011692: [测试群(860105388)]管理员/Alice(3001): 您好',
      },
    ]
    logsStore.selectedLogId = 'log_bridge_0003'
    logsStore.currentDetail = {
      log_id: 'log_bridge_0003',
      timestamp: '2026-04-09T10:27:21Z',
      level: 'info',
      protocol: 'onebot11',
      source: 'bridge',
      request_id: 'req_bridge_0003',
      message: '721011692: [测试群(860105388)]管理员/Alice(3001): 您好',
      details: {
        time: 'not-a-date',
      },
    }

    vi.spyOn(logsStore, 'fetchList').mockResolvedValue(logsStore.items)

    const router = createTestRouter()
    await router.push('/protocols/logs')
    await router.isReady()

    const wrapper = mount(ProtocolLogsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    logsStore.items = [
      ...logsStore.items,
      {
        log_id: 'log_bridge_0004',
        timestamp: 'not-a-date',
        level: 'info',
        protocol: 'onebot11',
        source: 'bridge',
        request_id: 'req_bridge_0004',
        message: '721011692: [测试群(860105388)]管理员/Alice(3001): 您好',
      },
    ]
    logsStore.selectedLogId = 'log_bridge_0004'
    logsStore.currentDetail = {
      log_id: 'log_bridge_0004',
      timestamp: 'not-a-date',
      level: 'info',
      protocol: 'onebot11',
      source: 'bridge',
      request_id: 'req_bridge_0004',
      message: '721011692: [测试群(860105388)]管理员/Alice(3001): 您好',
      details: {
        event_timestamp: Number.MAX_SAFE_INTEGER,
        time: 'not-a-date',
      },
    }

    await flushPromises()

    expect(wrapper.find('.logs-display-grid').exists()).toBe(true)
    expect(wrapper.text()).toContain('721011692: [测试群(860105388)]管理员/Alice(3001): 您好')
    expect(wrapper.text()).toContain('not-a-date')
  })

  it('toggles the protocol log stream activity when the cached page deactivates and reactivates', async () => {
    const logsStore = useProtocolLogsStore()

    vi.spyOn(logsStore, 'fetchList').mockResolvedValue([])

    const router = createTestRouter()
    await router.push('/protocols/logs')
    await router.isReady()

    const KeepAliveHarness = defineComponent({
      components: {
        PlaceholderView: { template: '<div>占位页</div>' },
        ProtocolLogsPage,
      },
      setup() {
        const currentView = ref<'logs' | 'placeholder'>('logs')
        return {
          currentView,
        }
      },
      template: `
        <KeepAlive>
          <component :is="currentView === 'logs' ? 'ProtocolLogsPage' : 'PlaceholderView'" />
        </KeepAlive>
      `,
    })

    const wrapper = mount(KeepAliveHarness, {
      attachTo: document.body,
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()
    expect(logsStore.active).toBe(true)

    wrapper.vm.currentView = 'placeholder'
    await flushPromises()
    expect(logsStore.active).toBe(false)

    wrapper.vm.currentView = 'logs'
    await flushPromises()
    expect(logsStore.active).toBe(true)
  })
})
