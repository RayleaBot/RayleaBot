import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import ThirdPartyMonitoringPage from '@/views/builtin/ThirdPartyMonitoringView.vue'
import { useThirdPartyMonitoringStore } from '@/stores/third-party-monitoring'
import type {
  BilibiliSourceStatusResponse,
  ThirdPartyMonitorItem,
  ThirdPartyMonitorsResponse,
} from '@/types/api'

vi.mock('@/adapter/feedback', () => ({
  notifyError: vi.fn(),
  notifySuccess: vi.fn(),
  useToastFeedback: vi.fn(),
}))

function createMonitoringRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/third-party-monitoring', name: 'third-party-monitoring', component: ThirdPartyMonitoringPage },
      { path: '/third-party-accounts', name: 'third-party-accounts', component: { template: '<div>accounts</div>' } },
    ],
  })
}

function monitorItem(): ThirdPartyMonitorItem {
  return {
    uid: '123456',
    username: '测试 UP',
    avatar_url: '',
    services: ['live', 'video'],
    dynamic: {
      last_id: '90001',
      service: 'video',
      title: '新视频标题',
      summary: '视频简介',
      url: 'https://www.bilibili.com/video/BV1RayleaBot',
      images: [],
      published_at: null,
      observed_at: '2026-06-08T08:11:05Z',
    },
    live: {
      room_id: '10001',
      room_name: '直播间标题',
      room_url: 'https://live.bilibili.com/10001',
      cover_url: '',
      is_live: false,
      live_started_at: null,
      live_ended_at: null,
      connection_state: 'degraded',
      last_error: '',
      updated_at: '2026-06-08T08:11:05Z',
    },
    updated_at: '2026-06-08T08:11:05Z',
  }
}

function monitorsResponse(items: ThirdPartyMonitorItem[] = [monitorItem()]): ThirdPartyMonitorsResponse {
  return {
    platform: 'bilibili',
    items,
    updated_at: '2026-06-08T08:11:05Z',
  }
}

function sourceStatus(overrides: Partial<BilibiliSourceStatusResponse> = {}): BilibiliSourceStatusResponse {
  return {
    status: 'degraded',
    summary: 'Bilibili 事件源运行受限',
    live: {
      watched_rooms: 1,
      connected_rooms: 0,
      failed_rooms: 1,
      fallback_polling: true,
      last_event_at: null,
      last_error: 'code -352',
    },
    dynamic: {
      enabled: true,
      interval_seconds: 10,
      watched_uids: 1,
      auto_follow: true,
      last_poll_at: '2026-06-08T08:10:05Z',
      last_event_at: '2026-06-08T08:09:58Z',
      last_error: '',
    },
    diagnosis: {
      level: 'attention',
      headline: '平台风控等待中',
      description: 'Bilibili 暂时限制直播请求，系统会在等待结束后自动恢复检查。',
      causes: [
        {
          scope: 'live',
          code: 'platform_risk_control',
          title: '直播请求被平台限制',
          detail: '直播状态检查暂时等待平台恢复。',
          last_error: 'code -352',
          retry_at: '2026-06-08T08:35:00Z',
        },
      ],
      impacts: [
        '直播状态暂时等待平台恢复。',
        '动态接收不受影响。',
        'CK 有效，无需重新登录。',
      ],
      actions: [
        { kind: 'wait', label: '等待平台恢复', target: null, primary: true },
        { kind: 'refresh', label: '刷新状态', target: null, primary: false },
      ],
      updated_at: '2026-06-08T08:30:00Z',
    },
    accounts: [
      {
        platform: 'bilibili',
        account_id: 'primary',
        label: '主账号',
        enabled: true,
        configured: true,
        profile: {
          uid: '123456',
          nickname: '主账号昵称',
          avatar_url: '',
        },
        credential: {
          state: 'valid',
          checked_at: '2026-06-08T08:00:01Z',
          last_error: '',
        },
        polling: {
          enabled: true,
          last_used_at: '2026-06-08T08:10:05Z',
        },
        updated_at: '2026-06-08T08:00:00Z',
      },
    ],
    ...overrides,
  }
}

async function mountMonitoringPage(status: BilibiliSourceStatusResponse = sourceStatus()) {
  const router = createMonitoringRouter()
  await router.push('/third-party-monitoring')
  await router.isReady()

  const store = useThirdPartyMonitoringStore()
  store.monitors = monitorsResponse()
  store.bilibiliStatus = status
  vi.spyOn(store, 'fetchAll').mockResolvedValue(undefined)
  vi.spyOn(store, 'restartBilibiliSource').mockResolvedValue({ accepted: true, status })

  const wrapper = mount(ThirdPartyMonitoringPage, {
    global: {
      plugins: [Antd, router],
    },
  })
  await flushPromises()
  return { router, wrapper }
}

describe('ThirdPartyMonitoringPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders the diagnosis bar without the internal degraded-check wording', async () => {
    const { wrapper } = await mountMonitoringPage()

    expect(wrapper.text()).toContain('运行受限')
    expect(wrapper.text()).toContain('原因')
    expect(wrapper.text()).toContain('影响')
    expect(wrapper.text()).toContain('处理')
    expect(wrapper.text()).toContain('平台风控等待中')
    expect(wrapper.text()).toContain('直播请求被平台限制')
    expect(wrapper.text()).toContain('动态接收不受影响')
    expect(wrapper.text()).toContain('CK 有效')
    expect(wrapper.text()).not.toContain('降级检查')
    expect(wrapper.text()).not.toContain('查看 Bilibili CK')
  })

  it('opens the account page only when CK handling is required', async () => {
    const { router, wrapper } = await mountMonitoringPage(sourceStatus({
      status: 'failed',
      diagnosis: {
        level: 'action_required',
        headline: 'CK 需要重新登录',
        description: 'Bilibili CK 无效，直播和动态检查需要可用 CK。',
        causes: [
          {
            scope: 'account',
            code: 'credential_invalid',
            title: 'CK 无效',
            detail: '主账号的 CK 无效。',
            last_error: '账号未登录',
            retry_at: null,
          },
        ],
        impacts: ['直播状态无法可靠检查。', '动态接收会受影响。', '需要重新获取 Bilibili CK。'],
        actions: [
          { kind: 'open_accounts', label: '查看 Bilibili CK', target: '/third-party-accounts', primary: true },
          { kind: 'refresh', label: '刷新状态', target: null, primary: false },
        ],
        updated_at: '2026-06-08T08:30:00Z',
      },
    }))

    const openButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('查看 Bilibili CK'))
    expect(openButton).toBeTruthy()
    await openButton!.trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.name).toBe('third-party-accounts')
  })
})
