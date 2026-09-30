import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppSelect from '@/components/AppSelect.vue'

describe('AppSelect in a filter toolbar', () => {
  const options = ['debug', 'info', 'warn', 'error'].map(value => ({ value, label: value }))

  it('names a full selection instead of listing every option', () => {
    const wrapper = mount(AppSelect, {
      props: { options, multiple: true, singleLine: true, allLabel: '全部', modelValue: ['debug', 'info', 'warn', 'error'] },
    })

    expect(wrapper.get('.app-select__value').text()).toBe('全部')
    expect(wrapper.get('.app-select__value').attributes('title')).toBe('debug、info、warn、error')
  })

  it('lists a partial selection on one line', () => {
    const wrapper = mount(AppSelect, {
      props: { options, multiple: true, singleLine: true, allLabel: '全部', modelValue: ['info', 'warn'] },
    })

    expect(wrapper.get('.app-select__value').text()).toBe('info、warn')
    expect(wrapper.get('.app-select__value').classes()).toContain('app-select__value--single-line')
  })
})
