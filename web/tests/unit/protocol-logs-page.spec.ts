import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import ProtocolLogsPage from '@/pages/ProtocolLogsPage.vue'
import { useProtocolLogsStore } from '@/stores/protocol-logs'

describe('ProtocolLogsPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Object.defineProperty(HTMLElement.prototype, 'scrollTo', {
      configurable: true,
      writable: true,
      value: vi.fn(),
    })
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
        plugins: [ElementPlus, router],
      },
    })

    await flushPromises()

    expect(HTMLElement.prototype.scrollTo).toHaveBeenCalledWith(expect.objectContaining({
      behavior: 'auto',
    }))
    expect(wrapper.text()).toContain('协议日志')
    expect(wrapper.find('.protocol-terminal').exists()).toBe(true)
    expect(wrapper.findAll('.protocol-terminal-line')).toHaveLength(1)
    expect(wrapper.text()).toContain('ignored OneBot API response with unsupported echo')
    expect(wrapper.text()).toContain('消息详情')
    expect(wrapper.text()).toContain('api.response.ignored')
    expect(wrapper.find('.detail-json-block').text()).toContain('"echo": 123')
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
        message: 'runtime bridge delivered adapter event',
      },
    ]

    vi.spyOn(logsStore, 'fetchList').mockResolvedValue(logsStore.items)
    const selectSpy = vi.spyOn(logsStore, 'selectLog').mockResolvedValue(null)

    const router = createTestRouter()
    await router.push('/protocols/logs')
    await router.isReady()

    const wrapper = mount(ProtocolLogsPage, {
      global: {
        plugins: [ElementPlus, router],
      },
    })

    await flushPromises()

    await wrapper.find('.protocol-terminal-line').trigger('click')
    expect(selectSpy).toHaveBeenCalledWith('log_bridge_0001')
  })

  it('uses smooth follow only for live append auto-follow updates', async () => {
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

    mount(ProtocolLogsPage, {
      global: {
        plugins: [ElementPlus, router],
      },
    })

    await flushPromises()
    vi.mocked(HTMLElement.prototype.scrollTo).mockClear()

    logsStore.items = [
      ...logsStore.items,
      {
        log_id: 'log_adapter_live_0002',
        timestamp: '2026-04-08T10:17:00Z',
        level: 'warn',
        protocol: 'onebot11',
        source: 'adapter.onebot11',
        request_id: 'req_adapter_ignored_0002',
        message: 'ignored OneBot API response with blank echo',
      },
    ]
    logsStore.selectedLogId = 'log_adapter_live_0002'

    await flushPromises()

    expect(HTMLElement.prototype.scrollTo).toHaveBeenCalledWith(expect.objectContaining({
      behavior: 'smooth',
    }))
  })
})
