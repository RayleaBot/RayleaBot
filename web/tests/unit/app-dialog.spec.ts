import { mount } from '@vue/test-utils'
import { defineComponent, h, nextTick } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

import AppDialog from '@/components/AppDialog.vue'

// A motion element that never reports a finished animation, as motion does when a change has nothing to animate.
vi.mock('motion-v', () => {
  const silent = (tag: string) => defineComponent({
    inheritAttrs: false,
    setup(_, { attrs, slots }) {
      return () => h(tag, attrs, slots.default?.())
    },
  })
  return { motion: { div: silent('div'), section: silent('section') } }
})

describe('AppDialog', () => {
  afterEach(() => {
    vi.useRealTimers()
    document.body.innerHTML = ''
  })

  it('finishes closing when the exit animation reports no completion', async () => {
    vi.useFakeTimers()
    const wrapper = mount(AppDialog, { props: { open: true, title: '移除条目' }, attachTo: document.body })
    await nextTick()
    expect(document.querySelector('.app-dialog-overlay')).not.toBeNull()

    await wrapper.setProps({ open: false })
    await vi.advanceTimersByTimeAsync(1000)

    expect(wrapper.emitted('afterClose')).toHaveLength(1)
    expect(document.querySelector('.app-dialog-overlay')).toBeNull()
    wrapper.unmount()
  })
})
