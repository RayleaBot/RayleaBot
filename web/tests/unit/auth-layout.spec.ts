import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import AuthLayout from '@/layouts/AuthLayout.vue'
import { useUiShellStore } from '@/stores/ui-shell'

describe('AuthLayout', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders a panel-right auth shell with toolbar controls', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        {
          path: '/auth',
          component: AuthLayout,
          children: [
            {
              path: '/login',
              component: {
                template: '<a-card><h1>登录</h1><p>输入管理员账号和密钥后进入管理界面。</p></a-card>',
              },
            },
          ],
        },
      ],
    })
    await router.push('/login')
    await router.isReady()

    const wrapper = mount(AuthLayout, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    const shellStore = useUiShellStore()

    expect(wrapper.find('.auth-layout__hero').exists()).toBe(true)
    expect(wrapper.find('.auth-layout__panel').exists()).toBe(true)
    expect(wrapper.get('[data-testid="auth-language"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="auth-theme-toggle"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('统一管理机器人、插件与协议')
    expect(wrapper.text()).toContain('登录')

    await wrapper.get('[data-testid="auth-theme-toggle"]').trigger('click')
    expect(shellStore.themeMode).toBe('dark')
  })
})
