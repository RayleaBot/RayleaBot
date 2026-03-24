import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { ApiError } from '@/lib/http'
import LoginPage from '@/pages/LoginPage.vue'
import { useSessionStore } from '@/stores/session'

describe('LoginPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('shows a short launcher admission fallback hint when present', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/login', component: LoginPage }],
    })
    await router.push('/login')
    await router.isReady()

    const sessionStore = useSessionStore()
    ;(sessionStore as any).launcherAdmissionHint = '自动登录未完成，请手动登录。'

    const wrapper = mount(LoginPage, {
      global: {
        plugins: [ElementPlus, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('自动登录未完成，请手动登录。')
    expect(wrapper.text()).not.toContain('session token surface')
  })

  it('shows a visible chinese error when login fails', async () => {
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
        plugins: [ElementPlus, router],
      },
    })

    const inputs = wrapper.findAll('input')
    await inputs[0].setValue('admin')
    await inputs[1].setValue('wrong-secret')
    await wrapper.get('.auth-submit').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('登录未完成，请检查管理员账号和密钥。')
  })

  it('shows a short chinese bootstrap hint when status is unavailable', async () => {
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
        plugins: [ElementPlus, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('暂时无法确认管理界面状态，请稍后重试。')
  })
})
