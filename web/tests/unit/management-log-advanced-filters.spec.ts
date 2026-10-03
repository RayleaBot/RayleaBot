import { createPinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { defineComponent, h } from 'vue'

import ManagementLogAdvancedFilters from '@/components/logs/ManagementLogAdvancedFilters.vue'

const PopoverStub = defineComponent({
  name: 'AppPopover',
  props: {
    open: Boolean,
  },
  emits: ['update:open'],
  setup(props, { emit, slots }) {
    return () => h('div', {
      class: 'popover-stub',
      onClick: () => emit('update:open', !props.open),
    }, [
      slots.default?.(),
      props.open ? slots.content?.() : null,
    ])
  },
})

describe('ManagementLogAdvancedFilters', () => {

  it('offers only the filters that did not fit in the toolbar, once requested', async () => {
    const wrapper = mount(ManagementLogAdvancedFilters, {
      attachTo: document.body,
      global: {
        plugins: [createPinia()],
        stubs: {
          AppPopover: PopoverStub,
        },
      },
      props: {
        fields: ['plugin', 'requestId'],
        modelValue: {},
      },
    })

    expect(document.body.querySelector('.log-advanced-filters__panel')).toBeNull()

    await wrapper.get('button').trigger('click')
    await flushPromises()

    const panel = document.body.querySelector('.log-advanced-filters__panel')
    expect(panel).not.toBeNull()
    expect([...panel!.querySelectorAll('.app-field__label')].map(label => label.textContent?.trim())).toEqual(['插件', '请求 ID'])
  })

  it('counts only the active filters it holds, since the others are in view', () => {
    const wrapper = mount(ManagementLogAdvancedFilters, {
      global: {
        plugins: [createPinia()],
      },
      props: {
        fields: ['plugin', 'requestId'],
        modelValue: { protocol: 'onebot11', pluginIds: ['weather'], requestId: 'req_1' },
      },
    })

    expect(wrapper.get('.log-advanced-filters__count').text()).toBe('2')
  })
})
