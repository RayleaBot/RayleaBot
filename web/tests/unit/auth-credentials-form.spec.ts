import Antd from 'ant-design-vue'
import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'

import AuthCredentialsForm from '@/components/auth/AuthCredentialsForm.vue'

function mountForm(options?: { pending?: boolean, feedback?: { level: 'error' | 'warning', message: string } | null }) {
  return mount(AuthCredentialsForm, {
    attachTo: document.body,
    props: {
      feedback: options?.feedback,
      pending: options?.pending ?? false,
      secretAutocomplete: 'current-password' as const,
      submitLabel: '登录',
      subtitle: '使用管理员账号和密钥进入管理工作区。',
      title: '登录',
    },
    global: {
      plugins: [Antd],
    },
  })
}

describe('AuthCredentialsForm', () => {
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('blocks submit, reports required fields, and focuses the first invalid input', async () => {
    const wrapper = mountForm()
    const inputs = wrapper.findAll('input')
    expect((inputs[0].element as HTMLInputElement).value).toBe('admin')

    await inputs[0].setValue('')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('submit')).toBeUndefined()
    expect(wrapper.text()).toContain('请输入管理员账号')
    expect(wrapper.text()).toContain('请输入管理员密钥')
    expect(inputs[0].attributes('aria-describedby')).toBe('auth-identifier-error')
    expect(inputs[1].attributes('aria-describedby')).toBe('auth-secret-error')
    expect(document.activeElement).toBe(inputs[0].element)
  })

  it('emits credentials and change events with the expected autocomplete semantics', async () => {
    const wrapper = mountForm()
    const inputs = wrapper.findAll('input')
    expect(inputs[0].attributes('autocomplete')).toBe('username')
    expect(inputs[1].attributes('autocomplete')).toBe('current-password')

    await inputs[1].setValue('super-secret')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('change')).toHaveLength(1)
    expect(wrapper.emitted('submit')).toEqual([[{ identifier: 'admin', secret: 'super-secret' }]])
  })

  it('provides a keyboard-operable password visibility control', async () => {
    const wrapper = mountForm()
    const input = wrapper.get('input[name="secret"]')
    const toggle = wrapper.get('button[aria-label="显示密钥"]')
    expect(input.attributes('type')).toBe('password')
    expect(toggle.attributes('aria-pressed')).toBe('false')

    await toggle.trigger('click')

    expect(input.attributes('type')).toBe('text')
    expect(wrapper.get('button[aria-label="隐藏密钥"]').attributes('aria-pressed')).toBe('true')
  })

  it('shows form feedback and disables all controls while pending', () => {
    const wrapper = mountForm({
      pending: true,
      feedback: { level: 'warning', message: '暂时无法确认管理界面状态，请稍后重试。' },
    })

    expect(wrapper.get('[role="status"]').text()).toContain('暂时无法确认管理界面状态')
    expect(wrapper.findAll('input').every((input) => input.attributes('disabled') !== undefined)).toBe(true)
    expect(wrapper.get('.auth-form__submit').attributes('aria-busy')).toBe('true')
    expect(wrapper.find('.ant-btn-loading-icon').exists()).toBe(true)
  })
})
