import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

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
})
