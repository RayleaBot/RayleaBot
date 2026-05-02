import Antd from 'ant-design-vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import VbenFallback from '@/components/fallback/VbenFallback.vue'

describe('VbenFallback', () => {
  it.each([
    ['403', '哎呀！访问被拒绝'],
    ['404', '哎呀！未找到页面'],
    ['500', '哎呀！出错了'],
    ['offline', '哎呀！网络错误'],
  ] as const)('renders the Vben fallback for %s', (status, title) => {
    const wrapper = mount(VbenFallback, {
      props: { status },
      global: {
        plugins: [Antd],
      },
    })

    expect(wrapper.get('[data-testid="vben-fallback"]').attributes('data-status')).toBe(status)
    expect(wrapper.text()).toContain(title)
    expect(wrapper.text()).toContain('返回首页')
  })

  it('uses a recheck action on the offline page', () => {
    const wrapper = mount(VbenFallback, {
      props: { status: 'offline' },
      global: {
        plugins: [Antd],
      },
    })

    expect(wrapper.text()).toContain('重新检测')
  })
})
