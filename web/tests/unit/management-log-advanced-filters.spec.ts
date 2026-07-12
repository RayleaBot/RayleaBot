import Antd from 'ant-design-vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import { defineComponent, h } from 'vue'

import ManagementLogAdvancedFilters from '@/components/logs/ManagementLogAdvancedFilters.vue'

const PopoverStub = defineComponent({
  name: 'APopover',
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
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('keeps low-frequency filters out of the toolbar flow until requested', async () => {
    const wrapper = mount(ManagementLogAdvancedFilters, {
      attachTo: document.body,
      global: {
        plugins: [Antd],
        stubs: {
          APopover: PopoverStub,
        },
      },
      props: {
        pluginOptions: [{ label: 'Weather', value: 'weather' }],
      },
    })

    expect(wrapper.find('details').exists()).toBe(false)
    expect(document.body.querySelector('.log-advanced-filters__panel')).toBeNull()

    await wrapper.get('button').trigger('click')
    await flushPromises()

    expect(document.body.querySelector('.log-advanced-filters__panel')).not.toBeNull()
  })

  it('shows how many advanced filter categories are active', () => {
    const wrapper = mount(ManagementLogAdvancedFilters, {
      global: {
        plugins: [Antd],
      },
      props: {
        pluginOptions: [{ label: 'Weather', value: 'weather' }],
        protocol: 'onebot11',
        pluginIds: ['weather'],
        requestId: 'req_1',
      },
    })

    expect(wrapper.get('.ant-badge-count').text()).toBe('3')
    expect(wrapper.get('button').classes()).toContain('is-active')
  })
})
