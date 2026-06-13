import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AuthTextField from '@/components/auth/AuthTextField.vue'

describe('AuthTextField', () => {
  it('associates the label with the input and emits v-model updates', async () => {
    const wrapper = mount(AuthTextField, {
      props: {
        label: '管理员账号',
        name: 'identifier',
        modelValue: '',
      },
    })

    const input = wrapper.get('input')
    const label = wrapper.get('label')
    expect(label.text()).toBe('管理员账号')
    expect(label.attributes('for')).toBeTruthy()
    expect(label.attributes('for')).toBe(input.attributes('id'))

    await input.setValue('admin')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual(['admin'])
  })

  it('toggles secret visibility with an accessible button', async () => {
    const wrapper = mount(AuthTextField, {
      props: {
        label: '管理员密钥',
        name: 'secret',
        type: 'password',
        modelValue: '',
      },
    })

    expect(wrapper.get('input').attributes('type')).toBe('password')
    const eye = wrapper.get('.auth-field__eye')
    expect(eye.attributes('aria-label')).toBe('显示密钥')
    expect(eye.attributes('aria-pressed')).toBe('false')

    await eye.trigger('click')

    expect(wrapper.get('input').attributes('type')).toBe('text')
    expect(eye.attributes('aria-label')).toBe('隐藏密钥')
    expect(eye.attributes('aria-pressed')).toBe('true')
  })

  it('exposes the error state to assistive technology', () => {
    const wrapper = mount(AuthTextField, {
      props: {
        label: '管理员密钥',
        name: 'secret',
        modelValue: '',
        error: '请输入管理员密钥',
      },
    })

    const input = wrapper.get('input')
    const alert = wrapper.get('[role="alert"]')
    expect(alert.text()).toBe('请输入管理员密钥')
    expect(input.attributes('aria-invalid')).toBe('true')
    expect(input.attributes('aria-describedby')).toBe(alert.attributes('id'))
  })
})
