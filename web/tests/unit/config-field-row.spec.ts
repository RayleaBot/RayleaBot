import Antd from 'ant-design-vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ConfigFieldRow from '@/components/config/ConfigFieldRow.vue'
import type { ConfigFieldDefinition } from '@/lib/config-form'

function mountField(field: ConfigFieldDefinition, value: unknown) {
  return mount(ConfigFieldRow, {
    props: { field, value },
    global: { plugins: [Antd] },
  })
}

describe('ConfigFieldRow', () => {
  it('emits the typed value for text fields', async () => {
    const wrapper = mountField({ path: 'server.host', label: 'host', type: 'text' }, '127.0.0.1')
    await wrapper.find('input').setValue('0.0.0.0')
    expect(wrapper.emitted('update:value')?.[0]).toEqual(['0.0.0.0'])
  })

  it('emits undefined when number input is cleared', async () => {
    const wrapper = mountField({ path: 'server.port', label: 'port', type: 'number' }, 8080)
    const input = wrapper.find('.config-field__number input')
    expect(input.exists()).toBe(true)
    await input.setValue('')
    expect(wrapper.emitted('update:value')?.at(-1)).toEqual([undefined])
  })

  it('emits a boolean for switch fields', async () => {
    const wrapper = mountField({ path: 'admin.sliding_renewal', label: 'sliding', type: 'boolean' }, false)
    await wrapper.find('.ant-switch').trigger('click')
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
    const vm = wrapper.vm as unknown as { emitSelect?: (value: unknown) => void }
    // simulate Ant Design Vue Select change via direct emit invocation through component instance
    const selectStub = wrapper.findComponent({ name: 'ASelect' })
    if (selectStub.exists()) {
      selectStub.vm.$emit('update:value', 'debug')
    } else {
      vm.emitSelect?.('debug')
    }
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

})
