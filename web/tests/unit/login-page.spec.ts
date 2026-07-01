import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { ApiError } from '@/lib/http'
import LoginPage from '@/views/auth/LoginView.vue'
import { useSessionStore } from '@/stores/session'

const feedbackMock = vi.hoisted(() => ({
  notifyError: vi.fn(),
  notifyWarning: vi.fn(),
}))

vi.mock('@/adapter/feedback', () => ({
  notifyError: feedbackMock.notifyError,
  notifySuccess: vi.fn(),
  notifyInfo: vi.fn(),
  notifyWarning: feedbackMock.notifyWarning,
  useToastFeedback: vi.fn((source: () => { level: 'error' | 'info' | 'success' | 'warning', message?: string | null } | null | undefined) => {
    const feedback = source()
    if (feedback?.level === 'warning' && feedback.message) {
      feedbackMock.notifyWarning(feedback.message)
    }
  }),
}))

describe('LoginPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    feedbackMock.notifyError.mockClear()
    feedbackMock.notifyWarning.mockClear()
  })

  it('shows a toast when login fails', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/login', component: LoginPage }],
    })
    await router.push('/login')
    await router.isReady()

    const sessionStore = useSessionStore()
    vi.spyOn(sessionStore, 'login').mockRejectedValue(
      new ApiError('当前用户无权执行该操作', 403, 'permission.denied'),
    )

    const wrapper = mount(LoginPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    const inputs = wrapper.findAll('input')
    await inputs[0].setValue('admin')
    await inputs[1].setValue('wrong-secret')
    await wrapper.get('.auth-submit').trigger('click')
    await flushPromises()

    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(feedbackMock.notifyError).toHaveBeenCalledTimes(1)
  })

  it('shows a toast when status is unavailable', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/login', component: LoginPage }],
    })
    await router.push('/login')
    await router.isReady()

    const sessionStore = useSessionStore()
    ;(sessionStore as any).bootstrapError = '暂时无法确认管理界面状态，请稍后重试。'

    const wrapper = mount(LoginPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(feedbackMock.notifyWarning).toHaveBeenCalledTimes(1)
  })
})
