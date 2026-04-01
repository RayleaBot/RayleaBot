import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import VirtualDataViewport from '@/components/VirtualDataViewport.vue'

describe('VirtualDataViewport', () => {
  beforeEach(() => {
    const height = 420
    const originalGetBoundingClientRect = HTMLElement.prototype.getBoundingClientRect

    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function mockGetBoundingClientRect() {
      if (this.classList?.contains('data-viewport__scroller')) {
        return {
          width: 960,
          height,
          x: 0,
          y: 0,
          top: 0,
          left: 0,
          bottom: height,
          right: 960,
          toJSON: () => ({}),
        } as DOMRect
      }

      return originalGetBoundingClientRect.call(this)
    })

    class ResizeObserverMock {
      callback: ResizeObserverCallback

      constructor(callback: ResizeObserverCallback) {
        this.callback = callback
      }

      observe(target: Element) {
        this.callback([
          {
            target,
            contentRect: {
              width: 960,
              height,
              x: 0,
              y: 0,
              top: 0,
              left: 0,
              bottom: height,
              right: 960,
              toJSON: () => ({}),
            },
          } as ResizeObserverEntry,
        ], this as unknown as ResizeObserver)
      }

      unobserve() {}
      disconnect() {}
    }

    window.ResizeObserver = ResizeObserverMock as typeof ResizeObserver
  })

  it('uses measured container height when viewportHeight is omitted', async () => {
    const items = Array.from({ length: 20 }, (_, index) => ({ id: `row-${index}` }))

    const wrapper = mount(VirtualDataViewport, {
      props: {
        items,
        itemHeight: 100,
        overscan: 1,
        getItemKey: (item: { id: string }) => item.id,
      },
      slots: {
        default: ({ item }: { item: { id: string } }) => item.id,
      },
    })

    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(wrapper.find('.data-viewport__scroller').attributes('style') ?? '').not.toContain('560px')
    expect(wrapper.findAll('.data-viewport__row')).toHaveLength(7)
  })
})
