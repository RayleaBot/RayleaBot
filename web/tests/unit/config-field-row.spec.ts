import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ConfigFieldRow from '@/components/config/ConfigFieldRow.vue'
import type { ConfigFieldDefinition } from '@/lib/config-form'

function mountField(field: ConfigFieldDefinition, value: unknown) {
  return mount(ConfigFieldRow, {
    props: { field, value },
  })
}

describe('ConfigFieldRow', () => {
  it.each([
    ['text', '127.0.0.1', 'input', '127.0.0.1'],
    ['number', 0, 'input', '0'],
    ['list', ['--disable-gpu'], 'textarea', '--disable-gpu'],
  ] as const)('shows the %s default only as a placeholder without writing it', (type, defaultValue, selector, placeholder) => {
    const wrapper = mountField({ path: 'fixture.value', label: 'Value', type, defaultValue }, undefined)
    const input = wrapper.get(selector)
    expect(input.attributes('placeholder')).toBe(placeholder)
    expect((input.element as HTMLInputElement | HTMLTextAreaElement).value).toBe('')
    expect(wrapper.emitted('update:value')).toBeUndefined()
    wrapper.unmount()
  })

  it('emits the typed value for text fields', async () => {
    const wrapper = mountField({ path: 'server.host', label: 'host', type: 'text' }, '127.0.0.1')
    await wrapper.find('input').setValue('0.0.0.0')
    expect(wrapper.emitted('update:value')?.[0]).toEqual(['0.0.0.0'])
  })

  it('shows separate default hints for an empty rate limit without filling either input', () => {
    const wrapper = mountField({ path: 'runtime.ipc_action_burst_limit', label: 'IPC', type: 'rateLimit', defaultValue: '100/1s' }, undefined)
    const inputs = wrapper.findAll('input')
    expect(inputs.map(input => input.attributes('placeholder'))).toEqual(['100', '1'])
    expect(inputs.map(input => (input.element as HTMLInputElement).value)).toEqual(['', ''])
    expect(wrapper.emitted('update:value')).toBeUndefined()
    wrapper.unmount()
  })

  it('preserves an incomplete rate-limit edit through remount and resets on discard', async () => {
    const field = { path: 'runtime.ipc_action_burst_limit', label: 'IPC', type: 'rateLimit' as const, defaultValue: '100/1s' }
    const wrapper = mountField(field, '100/1s')
    await wrapper.get('input[aria-label="IPC 次数"]').setValue('')
    const incomplete = wrapper.emitted('update:value')?.at(-1)?.[0]
    expect(incomplete).toBe('/1s')
    wrapper.unmount()
    const restored = mountField(field, incomplete)
    expect((restored.get('input[aria-label="IPC 次数"]').element as HTMLInputElement).value).toBe('')
    expect((restored.get('input[aria-label="IPC 时间窗口"]').element as HTMLInputElement).value).toBe('1')
    await restored.setProps({ value: '100/1s' })
    expect((restored.get('input[aria-label="IPC 次数"]').element as HTMLInputElement).value).toBe('100')
    restored.unmount()
  })

  it('emits undefined when number input is cleared', async () => {
    const wrapper = mountField({ path: 'server.port', label: 'port', type: 'number' }, 8080)
    const input = wrapper.find('input.config-field__number')
    expect(input.exists()).toBe(true)
    await input.setValue('')
    expect(wrapper.emitted('update:value')?.at(-1)).toEqual([undefined])
  })

  it('emits a boolean for switch fields', async () => {
    const wrapper = mountField({ path: 'admin.sliding_renewal', label: 'sliding', type: 'boolean' }, false)
    await wrapper.find('[role=switch]').trigger('click')
    expect(wrapper.emitted('update:value')?.[0]).toEqual([true])
  })

  it('emits the selected option for select fields', async () => {
    const wrapper = mountField(
      {
        path: 'log.level',
        label: 'level',
        type: 'select',
        options: [
          { label: 'Info', value: 'info' },
          { label: 'Debug', value: 'debug' },
        ],
      },
      'info',
    )
    const select = wrapper.getComponent({ name: 'AppSelect' })
    await select.vm.$emit('update:modelValue', 'debug')
    expect(wrapper.emitted('update:value')?.[0]).toEqual(['debug'])
  })

  it('splits multiline textarea into a list for list fields', async () => {
    const wrapper = mountField(
      { path: 'http.allow_private_hosts', label: 'hosts', type: 'list' },
      ['10.0.0.1'],
    )
    const textarea = wrapper.find('textarea')
    expect(textarea.exists()).toBe(true)
    await textarea.setValue('10.0.0.1\n10.0.0.2')
    expect(wrapper.emitted('update:value')?.at(-1)).toEqual([['10.0.0.1', '10.0.0.2']])
  })

  it('preserves boolean option types instead of stringifying false', async () => {
    const wrapper = mountField({ path: 'enabled', label: 'enabled', type: 'select', options: [
      { label: '启用', value: true }, { label: '关闭', value: false },
    ] }, false)
    const select = wrapper.getComponent({ name: 'AppSelect' })
    expect(select.props('modelValue')).toBe(false)
    expect(select.text()).toContain('关闭')
    await select.vm.$emit('update:modelValue', true)
    expect(wrapper.emitted('update:value')?.[0]).toEqual([true])
    wrapper.unmount()
  })

})
