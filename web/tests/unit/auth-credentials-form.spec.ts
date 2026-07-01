import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AuthCredentialsForm from '@/components/auth/AuthCredentialsForm.vue'

function mountForm(pending = false) {
  return mount(AuthCredentialsForm, {
    props: {
      title: '登录',
      subtitle: '输入管理员账号和密钥后进入管理界面。',
      submitLabel: '登录',
      pending,
      secretAutocomplete: 'current-password' as const,
    },
  })
}

describe('AuthCredentialsForm', () => {
  it('blocks submit and shows inline validation when the secret is empty', async () => {
    const wrapper = mountForm()

    const inputs = wrapper.findAll('input')
    expect((inputs[0].element as HTMLInputElement).value).toBe('admin')

    await wrapper.get('.auth-submit').trigger('click')

    expect(wrapper.emitted('submit')).toBeUndefined()
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
  })

  it('emits the credentials payload when fields are filled', async () => {
    const wrapper = mountForm()

    const inputs = wrapper.findAll('input')
    await inputs[1].setValue('super-secret')
    await wrapper.get('.auth-submit').trigger('click')

    expect(wrapper.emitted('submit')).toEqual([[{ identifier: 'admin', secret: 'super-secret' }]])
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })

  it('disables the submit button while pending', () => {
    const wrapper = mountForm(true)

    const submit = wrapper.get('.auth-submit')
    expect(submit.attributes('disabled')).toBeDefined()
    expect(submit.attributes('aria-busy')).toBe('true')
    expect(wrapper.find('.auth-submit__spinner').exists()).toBe(true)
  })
})
