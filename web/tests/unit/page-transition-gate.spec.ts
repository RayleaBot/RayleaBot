import { mount } from '@vue/test-utils'
import { defineComponent, h, provide, ref } from 'vue'
import { describe, expect, it } from 'vitest'
import { PAGE_TRANSITION_STAGE_KEY, useHeavyContentGate, type PageTransitionStage } from '@/layouts/usePageTransitionStage'

describe('heavy content lifetime', () => {
  it('settles pending work on unmount even if the enter transition never finishes', async () => {
    let gate!: ReturnType<typeof useHeavyContentGate>
    const Child = defineComponent({ setup() { gate = useHeavyContentGate(); return () => h('div') } })
    const Harness = defineComponent({ setup() { provide(PAGE_TRANSITION_STAGE_KEY, ref<PageTransitionStage>('entering')); return () => h(Child) } })
    const wrapper = mount(Harness)
    const waiting = gate.waitUntilReady()
    wrapper.unmount()
    await expect(waiting).resolves.toBe(false)
    await expect(gate.waitUntilReady()).resolves.toBe(false)
  })
})
