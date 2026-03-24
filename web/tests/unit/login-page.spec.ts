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

  it('shows a launcher admission expiry hint when present', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/login', component: LoginPage }],
    })
    await router.push('/login')
    await router.isReady()

    const sessionStore = useSessionStore()
    ;(sessionStore as any).launcherAdmissionHint = 'Launcher 登录令牌无效或已过期，请重新从启动器打开 Web UI。'

    const wrapper = mount(LoginPage, {
      global: {
        plugins: [ElementPlus, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Launcher 登录令牌无效或已过期')
  })
})
