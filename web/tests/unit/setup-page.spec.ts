import ElementPlus, { ElMessage } from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { ApiError } from '@/lib/http'
import SetupPage from '@/pages/SetupPage.vue'
import { useSessionStore } from '@/stores/session'

describe('SetupPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.spyOn(ElMessage, 'error').mockImplementation(() => undefined as never)
  })

  it('shows a visible chinese error when setup fails', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/setup', component: SetupPage }],
    })
    await router.push('/setup')
    await router.isReady()

    const sessionStore = useSessionStore()
    vi.spyOn(sessionStore, 'setupAdmin').mockRejectedValue(
      new ApiError('请求参数不合法', 400, 'platform.invalid_request'),
    )

    const wrapper = mount(SetupPage, {
      global: {
        plugins: [ElementPlus, router],
      },
    })

    const inputs = wrapper.findAll('input')
    await inputs[0].setValue('admin')
    await inputs[1].setValue('admin')
    await wrapper.get('.auth-submit').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('创建管理员账号未完成，请检查输入后重试。')
  })
})
