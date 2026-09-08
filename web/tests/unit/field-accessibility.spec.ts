import { h, type Component } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import AppField from '@/components/AppField.vue'
import AppInput from '@/components/AppInput.vue'
import AppNumberInput from '@/components/AppNumberInput.vue'
import AppTextarea from '@/components/AppTextarea.vue'
import AppSelect from '@/components/AppSelect.vue'
import AppTagsInput from '@/components/AppTagsInput.vue'
import AppSwitch from '@/components/AppSwitch.vue'

describe('field descriptions', () => {
  const controls: Array<[string, Component, Record<string, unknown>, string]> = [
    ['text', AppInput, { modelValue: '' }, 'input'],
    ['number', AppNumberInput, { modelValue: 1 }, 'input'],
    ['textarea', AppTextarea, { modelValue: '' }, 'textarea'],
    ['select', AppSelect, { modelValue: 'a', options: [{ value: 'a', label: 'A' }] }, '[role=combobox]'],
    ['tags', AppTagsInput, { modelValue: [] }, 'input'],
    ['switch', AppSwitch, { modelValue: false }, '[role=switch]'],
  ]

  it.each(controls)('associates the %s control with its label, hint and changing error', async (_name, component, props, selector) => {
    const wrapper = mount(AppField, {
      props: { label: '凭据', hint: '留空时保留已保存的凭据。' },
      slots: { default: () => h(component, props) },
    })
    const control = wrapper.get(selector)
    expect(wrapper.get('label').attributes('for')).toBe(control.attributes('id'))
    expect(control.attributes('aria-describedby')).toBe(wrapper.get('.app-field__description').attributes('id'))
    expect(control.attributes('aria-invalid')).toBeUndefined()
    await wrapper.setProps({ error: '凭据格式无效。' })
    expect(control.attributes('aria-describedby')).toBe(wrapper.get('[role=alert]').attributes('id'))
    expect(control.attributes('aria-invalid')).toBe('true')
    await wrapper.setProps({ hint: undefined, error: undefined })
    expect(control.attributes('aria-describedby')).toBeUndefined()
    expect(wrapper.find('.app-field__description').exists()).toBe(false)
    wrapper.unmount()
  })

  it.each(controls.slice(0, 4))('preserves the %s label and description inside a floating field', async (_name, component, props, selector) => {
    const wrapper = mount(AppField, {
      props: { label: '字段名称', floating: true, hint: '字段说明', required: true },
      slots: { default: () => h(component, props) },
    })
    const control = wrapper.get(selector)
    const label = wrapper.get('.app-field__floating-control > label')
    expect(label.attributes('for')).toBe(control.attributes('id'))
    expect(control.attributes('aria-describedby')).toBe(wrapper.get('.app-field__description').attributes('id'))
    expect(control.attributes('aria-required')).toBe('true')
    expect(wrapper.findAll('label')).toHaveLength(1)
    await wrapper.setProps({ error: '输入无效' })
    expect(control.attributes('aria-invalid')).toBe('true')
    expect(control.attributes('aria-describedby')).toBe(wrapper.get('[role=alert]').attributes('id'))
  })

  it.each([
    ['number', AppNumberInput, { modelValue: null, nullable: true, placeholder: undefined }, 'input'],
    ['textarea', AppTextarea, { modelValue: '', placeholder: '' }, 'textarea'],
  ] as const)('keeps the empty %s eligible for its resting label with an empty optional placeholder', (_name, component, props, selector) => {
    const wrapper = mount(AppField, {
      props: { label: '字段名称', floating: true },
      slots: { default: () => h(component as Component, props) },
    })
    const control = wrapper.get(selector)
    expect(control.attributes('placeholder')).toBe(' ')
    expect((control.element as HTMLInputElement | HTMLTextAreaElement).value).toBe('')
  })
})
