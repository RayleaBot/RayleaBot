import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import VirtualDataViewport from '@/components/VirtualDataViewport.vue'

const viewportHeight = 420
const fallbackRowHeight = 64
let heightByLabel = new Map<string, number>()

function createRect(height: number, width = 960) {
  return {
    width,
    height,
    x: 0,
    y: 0,
    top: 0,
    left: 0,
    bottom: height,
    right: width,
    toJSON: () => ({}),
  } as DOMRect
}

function getMeasuredHeight(element: HTMLElement) {
  if (element.classList.contains('data-viewport__scroller')) {
    return viewportHeight
  }

  if (element.classList.contains('data-viewport__row')) {
    const label = element.textContent?.trim() ?? ''
    return heightByLabel.get(label) ?? fallbackRowHeight
  }

  return 0
}

class ResizeObserverMock {
  static instances = new Set<ResizeObserverMock>()

  callback: ResizeObserverCallback

  constructor(callback: ResizeObserverCallback) {
    this.callback = callback
    ResizeObserverMock.instances.add(this)
  }

  observe(target: Element) {
    if (!(target instanceof HTMLElement)) {
      return
    }

    this.callback([
      {
        target,
        contentRect: createRect(getMeasuredHeight(target)),
      } as ResizeObserverEntry,
    ], this as unknown as ResizeObserver)
  }

  unobserve() {}

  disconnect() {
    ResizeObserverMock.instances.delete(this)
  }

  static trigger(target: Element) {
    if (!(target instanceof HTMLElement)) {
      return
    }

    for (const observer of ResizeObserverMock.instances) {
      observer.callback([
        {
          target,
          contentRect: createRect(getMeasuredHeight(target)),
        } as ResizeObserverEntry,
      ], observer as unknown as ResizeObserver)
    }
  }
}

function expectCanvasHeight(wrapper: VueWrapper, expectedHeight: number) {
  expect(wrapper.get('.data-viewport__canvas').attributes('style')).toContain(`height: ${expectedHeight}px;`)
}

describe('VirtualDataViewport', () => {
  beforeEach(() => {
    heightByLabel = new Map()
    const originalGetBoundingClientRect = HTMLElement.prototype.getBoundingClientRect

    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function mockGetBoundingClientRect() {
      if (this instanceof HTMLElement) {
        return createRect(getMeasuredHeight(this))
      }

      return originalGetBoundingClientRect.call(this)
    })

    window.ResizeObserver = ResizeObserverMock as typeof ResizeObserver
  })

  afterEach(() => {
    ResizeObserverMock.instances.clear()
    vi.restoreAllMocks()
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

  it('keeps prepended row measurements stable after ref cleanup and later resize updates', async () => {
    heightByLabel = new Map([
      ['A', 120],
      ['B', 80],
      ['C', 140],
    ])

    const wrapper = mount(VirtualDataViewport, {
      props: {
        items: [{ id: 'A' }, { id: 'B' }],
        itemHeight: 60,
        dynamicItemHeight: true,
        getItemKey: (item: { id: string }) => item.id,
      },
      slots: {
        default: ({ item }: { item: { id: string } }) => item.id,
      },
    })

    await flushPromises()
    await wrapper.vm.$nextTick()

    expectCanvasHeight(wrapper, 200)

    await wrapper.setProps({
      items: [{ id: 'C' }, { id: 'A' }, { id: 'B' }],
    })
    await flushPromises()
    await wrapper.vm.$nextTick()

    expectCanvasHeight(wrapper, 340)

    heightByLabel.set('C', 200)
    const prependedRow = wrapper.findAll('.data-viewport__row').find((row) => row.text().trim() === 'C')
    expect(prependedRow).toBeTruthy()

    ResizeObserverMock.trigger(prependedRow!.element)
    await wrapper.vm.$nextTick()

    expectCanvasHeight(wrapper, 400)
  })

  it('keeps the scroll anchor stable when a row above the viewport is remeasured', async () => {
    heightByLabel = new Map(
      Array.from({ length: 12 }, (_, index) => [`Row ${index}`, fallbackRowHeight]),
    )

    const wrapper = mount(VirtualDataViewport, {
      props: {
        items: Array.from({ length: 12 }, (_, index) => ({ id: `row-${index}`, label: `Row ${index}` })),
        itemHeight: fallbackRowHeight,
        dynamicItemHeight: true,
        overscan: 3,
        getItemKey: (item: { id: string }) => item.id,
      },
      slots: {
        default: ({ item }: { item: { label: string } }) => item.label,
      },
    })

    await flushPromises()
    await wrapper.vm.$nextTick()

    const scroller = wrapper.get('.data-viewport__scroller').element as HTMLElement
    scroller.scrollTop = 280
    await wrapper.get('.data-viewport__scroller').trigger('scroll')
    await wrapper.vm.$nextTick()

    const rowAboveViewport = wrapper.findAll('.data-viewport__row').find((row) => row.text().trim() === 'Row 1')
    expect(rowAboveViewport).toBeTruthy()

    heightByLabel.set('Row 1', 120)
    ResizeObserverMock.trigger(rowAboveViewport!.element)
    await wrapper.vm.$nextTick()

    expect(scroller.scrollTop).toBe(336)
    expectCanvasHeight(wrapper, 824)
  })
})
