import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { ApiError } from '@/lib/http'
import LoginPage from '@/views/auth/LoginView.vue'
import { useSessionStore } from '@/stores/session'

const feedbackMock = vi.hoisted(() => ({
  notifySuccess: vi.fn(),
}))

vi.mock('@/adapter/feedback', () => ({
  notifyError: vi.fn(),
  notifySuccess: feedbackMock.notifySuccess,
  notifyInfo: vi.fn(),
  notifyWarning: vi.fn(),
  useToastFeedback: vi.fn(),
}))

describe('LoginPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    feedbackMock.notifySuccess.mockClear()
  })

  it('shows form feedback when login fails and clears it when credentials change', async () => {
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
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(wrapper.get('[role="alert"]').text()).toContain('登录未完成，请检查管理员账号和密钥。')

    await inputs[1].setValue('next-secret')
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })

  it('shows persistent form feedback when status is unavailable', async () => {
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

    expect(wrapper.get('[role="status"]').text()).toContain('暂时无法确认管理界面状态，请稍后重试。')
  })
})
