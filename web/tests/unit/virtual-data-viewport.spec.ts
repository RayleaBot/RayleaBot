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

function rowStart(row: VueWrapper) {
  const transform = row.attributes('style') ?? ''
  return Number(/translateY\(([-\d.]+)px\)/.exec(transform)?.[1] ?? 0)
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
    vi.spyOn(window, 'requestAnimationFrame').mockImplementation((callback: FrameRequestCallback) => {
      callback(0)
      return 0
    })
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
    const renderedRows = wrapper.findAll('.data-viewport__row')
    expect(renderedRows.length).toBeGreaterThan(0)
    expect(renderedRows.length).toBeLessThan(items.length)
  })

  it('measures the viewport after rows appear from an initially empty state', async () => {
    const wrapper = mount(VirtualDataViewport, {
      props: {
        items: [],
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

    await wrapper.setProps({
      items: Array.from({ length: 20 }, (_, index) => ({ id: `row-${index}` })),
    })
    await flushPromises()
    await wrapper.vm.$nextTick()

    const renderedRows = wrapper.findAll('.data-viewport__row')
    expect(renderedRows.length).toBeGreaterThan(0)
    expect(renderedRows.length).toBeLessThan(20)
  })

  it('keeps near-integer dynamic row measurements on the estimated pixel grid', async () => {
    heightByLabel = new Map(
      Array.from({ length: 8 }, (_, index) => [`Row ${index}`, 80.05]),
    )

    const wrapper = mount(VirtualDataViewport, {
      props: {
        items: Array.from({ length: 8 }, (_, index) => ({ id: `row-${index}`, label: `Row ${index}` })),
        itemHeight: 80,
        viewportHeight: 420,
        dynamicItemHeight: true,
        overscan: 1,
        getItemKey: (item: { id: string }) => item.id,
      },
      slots: {
        default: ({ item }: { item: { label: string } }) => item.label,
      },
    })

    await flushPromises()
    await wrapper.vm.$nextTick()

    expectCanvasHeight(wrapper, 640)
    const rowStarts = wrapper.findAll('.data-viewport__row').slice(0, 4).map(rowStart)
    expect(rowStarts).toEqual([0, 80, 160, 240])
  })

  it('keeps true dynamic height for rows that are clearly taller than the estimate', async () => {
    heightByLabel = new Map([
      ['Short', 80.05],
      ['Long', 126.4],
      ['Next', 80.05],
    ])

    const wrapper = mount(VirtualDataViewport, {
      props: {
        items: [{ id: 'short', label: 'Short' }, { id: 'long', label: 'Long' }, { id: 'next', label: 'Next' }],
        itemHeight: 80,
        viewportHeight: 420,
        dynamicItemHeight: true,
        overscan: 1,
        getItemKey: (item: { id: string }) => item.id,
      },
      slots: {
        default: ({ item }: { item: { label: string } }) => item.label,
      },
    })

    await flushPromises()
    await wrapper.vm.$nextTick()

    const rowStarts = wrapper.findAll('.data-viewport__row').slice(0, 3).map(rowStart)
    expect(rowStarts[0]).toBe(0)
    expect(rowStarts[1]).toBe(80)
    expect(rowStarts[2]).toBeGreaterThan(205)
    expect(wrapper.get('.data-viewport__canvas').attributes('style')).toContain('height: 286.')
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

    expect(wrapper.get('.data-viewport__canvas').attributes('style')).toContain('height:')
  })

  it('allows reaching the top again after older rows are prepended without an intermediate scroll-away', async () => {
    heightByLabel = new Map([
      ['Older 1', 120],
      ['Older 2', 100],
      ['A', 120],
      ['B', 80],
      ['C', 140],
      ['D', 160],
    ])

    const wrapper = mount(VirtualDataViewport, {
      props: {
        items: [{ id: 'A' }, { id: 'B' }, { id: 'C' }, { id: 'D' }],
        itemHeight: 60,
        viewportHeight: 180,
        dynamicItemHeight: true,
        getItemKey: (item: { id: string }) => item.id,
      },
      slots: {
        default: ({ item }: { item: { id: string } }) => item.id,
      },
    })

    await flushPromises()
    await wrapper.vm.$nextTick()

    const scroller = wrapper.get('.data-viewport__scroller').element as HTMLElement
    let internalScrollTop = 0
    Object.defineProperty(scroller, 'clientHeight', {
      configurable: true,
      value: 180,
    })
    Object.defineProperty(scroller, 'scrollTop', {
      configurable: true,
      get: () => internalScrollTop,
      set: (value: number) => {
        internalScrollTop = Math.floor(value)
      },
    })
    Object.defineProperty(scroller, 'scrollHeight', {
      configurable: true,
      get: () => {
        const style = wrapper.get('.data-viewport__canvas').attributes('style')
        const matched = /height:\s*(\d+)px/.exec(style)
        return matched ? Number(matched[1]) : 0
      },
    })

    scroller.scrollTop = 48
    await wrapper.get('.data-viewport__scroller').trigger('scroll')
    await wrapper.vm.$nextTick()

    scroller.scrollTop = 0
    await wrapper.get('.data-viewport__scroller').trigger('scroll')
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('reach-top')).toHaveLength(1)

    const previousScrollHeight = scroller.scrollHeight

    await wrapper.setProps({
      items: [{ id: 'Older 1' }, { id: 'Older 2' }, { id: 'A' }, { id: 'B' }, { id: 'C' }, { id: 'D' }],
    })
    await flushPromises()
    await wrapper.vm.$nextTick()

    const nextScrollHeight = scroller.scrollHeight
    const expectedDelta = nextScrollHeight - previousScrollHeight
    expect(expectedDelta).toBeGreaterThan(0)
    expect(scroller.scrollTop).toBeGreaterThanOrEqual(expectedDelta - 2)
    expect(scroller.scrollTop).toBeLessThanOrEqual(expectedDelta + 2)

    scroller.scrollTop = 0
    await wrapper.get('.data-viewport__scroller').trigger('scroll')
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('reach-top')).toHaveLength(2)
  })

  it('advances scrollTop to anchor the prior viewport position after older rows are prepended', async () => {
    heightByLabel = new Map([
      ['Older 1', 120],
      ['Older 2', 100],
      ['A', 120],
      ['B', 80],
      ['C', 140],
    ])

    const wrapper = mount(VirtualDataViewport, {
      props: {
        items: [{ id: 'A' }, { id: 'B' }, { id: 'C' }],
        itemHeight: 60,
        viewportHeight: 180,
        dynamicItemHeight: true,
        getItemKey: (item: { id: string }) => item.id,
      },
      slots: {
        default: ({ item }: { item: { id: string } }) => item.id,
      },
    })

    await flushPromises()
    await wrapper.vm.$nextTick()

    const scroller = wrapper.get('.data-viewport__scroller').element as HTMLElement
    let internalScrollTop = 0
    Object.defineProperty(scroller, 'clientHeight', {
      configurable: true,
      value: 180,
    })
    Object.defineProperty(scroller, 'scrollTop', {
      configurable: true,
      get: () => internalScrollTop,
      set: (value: number) => {
        internalScrollTop = Math.floor(value)
      },
    })
    Object.defineProperty(scroller, 'scrollHeight', {
      configurable: true,
      get: () => {
        const style = wrapper.get('.data-viewport__canvas').attributes('style')
        const matched = /height:\s*(\d+)px/.exec(style)
        return matched ? Number(matched[1]) : 0
      },
    })

    scroller.scrollTop = 36
    await wrapper.get('.data-viewport__scroller').trigger('scroll')
    await wrapper.vm.$nextTick()

    scroller.scrollTop = 0
    await wrapper.get('.data-viewport__scroller').trigger('scroll')
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('reach-top')).toHaveLength(1)

    const previousScrollHeight = scroller.scrollHeight

    await wrapper.setProps({
      items: [{ id: 'Older 1' }, { id: 'Older 2' }, { id: 'A' }, { id: 'B' }, { id: 'C' }],
    })
    await flushPromises()
    await wrapper.vm.$nextTick()

    const nextScrollHeight = scroller.scrollHeight
    const expectedDelta = nextScrollHeight - previousScrollHeight
    expect(scroller.scrollTop).toBeGreaterThanOrEqual(expectedDelta - 2)
    expect(scroller.scrollTop).toBeLessThanOrEqual(expectedDelta + 2)

    for (let attempt = 0; attempt < 3; attempt += 1) {
      await wrapper.get('.data-viewport__scroller').trigger('scroll')
      await wrapper.vm.$nextTick()
    }

    expect(wrapper.emitted('reach-top')).toHaveLength(1)
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
    let internalScrollTop = 0
    Object.defineProperty(scroller, 'clientHeight', {
      configurable: true,
      value: viewportHeight,
    })
    Object.defineProperty(scroller, 'scrollTop', {
      configurable: true,
      get: () => internalScrollTop,
      set: (value: number) => {
        internalScrollTop = Math.floor(value)
      },
    })
    Object.defineProperty(scroller, 'scrollHeight', {
      configurable: true,
      get: () => {
        const style = wrapper.get('.data-viewport__canvas').attributes('style')
        const matched = /height:\s*(\d+)px/.exec(style)
        return matched ? Number(matched[1]) : 0
      },
    })
    scroller.scrollTop = 280
    await wrapper.get('.data-viewport__scroller').trigger('scroll')
    await wrapper.vm.$nextTick()

    const rowAboveViewport = wrapper.findAll('.data-viewport__row').find((row) => row.text().trim() === 'Row 1')
    expect(rowAboveViewport).toBeTruthy()

    heightByLabel.set('Row 1', 120)
    ResizeObserverMock.trigger(rowAboveViewport!.element)
    await wrapper.vm.$nextTick()

    expect(scroller.scrollTop).toBeGreaterThanOrEqual(280)
    expect(wrapper.get('.data-viewport__canvas').attributes('style')).toContain('height:')
  }, 15000)

  it('does not snap the viewport back when rows are measured for the first time while scrolling upward', async () => {
    heightByLabel = new Map(
      Array.from({ length: 30 }, (_, index) => [`Row ${index}`, 120]),
    )

    const wrapper = mount(VirtualDataViewport, {
      props: {
        items: Array.from({ length: 30 }, (_, index) => ({ id: `row-${index}`, label: `Row ${index}` })),
        itemHeight: fallbackRowHeight,
        dynamicItemHeight: true,
        overscan: 1,
        getItemKey: (item: { id: string }) => item.id,
      },
      slots: {
        default: ({ item }: { item: { label: string } }) => item.label,
      },
    })

    await flushPromises()
    await wrapper.vm.$nextTick()

    const scroller = wrapper.get('.data-viewport__scroller').element as HTMLElement
    Object.defineProperty(scroller, 'clientHeight', {
      configurable: true,
      value: viewportHeight,
    })
    Object.defineProperty(scroller, 'scrollHeight', {
      configurable: true,
      get: () => {
        const style = wrapper.get('.data-viewport__canvas').attributes('style')
        const matched = /height:\s*(\d+)px/.exec(style)
        return matched ? Number(matched[1]) : 0
      },
    })

    scroller.scrollTop = 1400
    await wrapper.get('.data-viewport__scroller').trigger('scroll')
    await wrapper.vm.$nextTick()

    scroller.scrollTop = 1240
    await wrapper.get('.data-viewport__scroller').trigger('scroll')
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(scroller.scrollTop).toBe(1240)
  })

  it('emits reach-top only when entering the top edge again', async () => {
    const wrapper = mount(VirtualDataViewport, {
      props: {
        items: Array.from({ length: 30 }, (_, index) => ({ id: `row-${index}`, label: `Row ${index}` })),
        itemHeight: fallbackRowHeight,
        viewportHeight,
        getItemKey: (item: { id: string }) => item.id,
      },
      slots: {
        default: ({ item }: { item: { label: string } }) => item.label,
      },
    })

    await flushPromises()
    await wrapper.vm.$nextTick()

    const scroller = wrapper.get('.data-viewport__scroller').element as HTMLElement

    scroller.scrollTop = 180
    await wrapper.get('.data-viewport__scroller').trigger('scroll')
    await wrapper.vm.$nextTick()

    scroller.scrollTop = 0
    await wrapper.get('.data-viewport__scroller').trigger('scroll')
    await wrapper.vm.$nextTick()

    scroller.scrollTop = 0
    await wrapper.get('.data-viewport__scroller').trigger('scroll')
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('reach-top')).toHaveLength(1)

    scroller.scrollTop = 40
    await wrapper.get('.data-viewport__scroller').trigger('scroll')
    await wrapper.vm.$nextTick()

    scroller.scrollTop = 0
    await wrapper.get('.data-viewport__scroller').trigger('scroll')
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('reach-top')).toHaveLength(2)
  })

  it('pauses bottom follow immediately when the user wheels upward', async () => {
    const wrapper = mount(VirtualDataViewport, {
      props: {
        items: Array.from({ length: 10 }, (_, index) => ({ id: `row-${index}`, label: `Row ${index}` })),
        itemHeight: fallbackRowHeight,
        viewportHeight,
        followBottom: false,
        getItemKey: (item: { id: string }) => item.id,
      },
      slots: {
        default: ({ item }: { item: { label: string } }) => item.label,
      },
    })

    await flushPromises()
    await wrapper.vm.$nextTick()

    const scroller = wrapper.get('.data-viewport__scroller').element as HTMLElement
    Object.defineProperty(scroller, 'clientHeight', {
      configurable: true,
      value: viewportHeight,
    })
    Object.defineProperty(scroller, 'scrollHeight', {
      configurable: true,
      get: () => (wrapper.props('items') as Array<unknown>).length * fallbackRowHeight,
    })

    await wrapper.setProps({ followBottom: true })
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(scroller.scrollTop).toBe(220)

    await wrapper.get('.data-viewport__scroller').trigger('wheel', {
      deltaY: -120,
    })
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('at-bottom-change')?.at(-1)).toEqual([false])

    await wrapper.setProps({
      items: Array.from({ length: 11 }, (_, index) => ({ id: `row-${index}`, label: `Row ${index}` })),
      followBottom: true,
    })
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(scroller.scrollTop).toBe(220)
  })

  it('does not snap back to the bottom when the user scrolls upward without any new rows', async () => {
    const wrapper = mount(VirtualDataViewport, {
      props: {
        items: Array.from({ length: 20 }, (_, index) => ({ id: `row-${index}`, label: `Row ${index}` })),
        itemHeight: fallbackRowHeight,
        viewportHeight,
        followBottom: true,
        getItemKey: (item: { id: string }) => item.id,
      },
      slots: {
        default: ({ item }: { item: { label: string } }) => item.label,
      },
    })

    await flushPromises()
    await wrapper.vm.$nextTick()

    const scroller = wrapper.get('.data-viewport__scroller').element as HTMLElement
    Object.defineProperty(scroller, 'clientHeight', {
      configurable: true,
      value: viewportHeight,
    })
    Object.defineProperty(scroller, 'scrollHeight', {
      configurable: true,
      get: () => (wrapper.props('items') as Array<unknown>).length * fallbackRowHeight,
    })

    await wrapper.setProps({ followBottom: false })
    await flushPromises()
    await wrapper.vm.$nextTick()

    await wrapper.setProps({ followBottom: true })
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(scroller.scrollTop).toBe(860)

    scroller.scrollTop = 600
    await wrapper.get('.data-viewport__scroller').trigger('scroll')
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(scroller.scrollTop).toBe(600)
    expect(wrapper.emitted('at-bottom-change')?.at(-1)).toEqual([false])
  })

  it('pins the viewport to the newest rows when follow mode is enabled', async () => {
    const wrapper = mount(VirtualDataViewport, {
      props: {
        items: Array.from({ length: 10 }, (_, index) => ({ id: `row-${index}`, label: `Row ${index}` })),
        itemHeight: fallbackRowHeight,
        viewportHeight,
        followBottom: false,
        getItemKey: (item: { id: string }) => item.id,
      },
      slots: {
        default: ({ item }: { item: { label: string } }) => item.label,
      },
    })

    await flushPromises()
    await wrapper.vm.$nextTick()

    const scroller = wrapper.get('.data-viewport__scroller').element as HTMLElement
    Object.defineProperty(scroller, 'clientHeight', {
      configurable: true,
      value: viewportHeight,
    })
    Object.defineProperty(scroller, 'scrollHeight', {
      configurable: true,
      get: () => (wrapper.props('items') as Array<unknown>).length * fallbackRowHeight,
    })

    await wrapper.setProps({ followBottom: true })
    await flushPromises()
    await wrapper.vm.$nextTick()

    const viewport = wrapper.vm as unknown as {
      getScrollMetrics: () => { scrollTop: number }
    }

    expect(viewport.getScrollMetrics().scrollTop).toBe(220)

    await wrapper.setProps({
      followBottom: false,
      items: Array.from({ length: 11 }, (_, index) => ({ id: `row-${index}`, label: `Row ${index}` })),
    })
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(viewport.getScrollMetrics().scrollTop).toBe(220)

    await wrapper.setProps({ followBottom: true })
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(viewport.getScrollMetrics().scrollTop).toBe(284)
  })

  it('keeps follow mode active after an appended row when the browser reports a fractional bottom gap', async () => {
    const wrapper = mount(VirtualDataViewport, {
      props: {
        items: Array.from({ length: 10 }, (_, index) => ({ id: `row-${index}`, label: `Row ${index}` })),
        itemHeight: fallbackRowHeight,
        viewportHeight,
        followBottom: false,
        bottomThreshold: 0,
        getItemKey: (item: { id: string }) => item.id,
      },
      slots: {
        default: ({ item }: { item: { label: string } }) => item.label,
      },
    })

    await flushPromises()
    await wrapper.vm.$nextTick()

    const scroller = wrapper.get('.data-viewport__scroller').element as HTMLElement
    let internalScrollTop = 0
    Object.defineProperty(scroller, 'clientHeight', {
      configurable: true,
      value: viewportHeight,
    })
    Object.defineProperty(scroller, 'scrollTop', {
      configurable: true,
      get: () => internalScrollTop,
      set: (value: number) => {
        internalScrollTop = Math.floor(value)
      },
    })
    Object.defineProperty(scroller, 'scrollHeight', {
      configurable: true,
      get: () => ((wrapper.props('items') as Array<unknown>).length * fallbackRowHeight) + 0.5,
    })

    const getFalseBottomChanges = () => (
      (wrapper.emitted('at-bottom-change') ?? []).filter((event) => event[0] === false).length
    )

    const falseChangesBeforeFollow = getFalseBottomChanges()

    await wrapper.setProps({ followBottom: true })
    await flushPromises()
    await wrapper.vm.$nextTick()
    await wrapper.get('.data-viewport__scroller').trigger('scroll')
    await wrapper.vm.$nextTick()

    const falseChangesAfterFollow = getFalseBottomChanges()

    await wrapper.setProps({
      items: Array.from({ length: 11 }, (_, index) => ({ id: `row-${index}`, label: `Row ${index}` })),
      followBottom: true,
    })
    await flushPromises()
    await wrapper.vm.$nextTick()
    await wrapper.get('.data-viewport__scroller').trigger('scroll')
    await wrapper.vm.$nextTick()

    expect(falseChangesAfterFollow).toBe(falseChangesBeforeFollow)
    expect(getFalseBottomChanges()).toBe(falseChangesAfterFollow)
    expect(internalScrollTop).toBeGreaterThanOrEqual(220)
  })

  it('clamps a stale scroll position when the available rows shrink', async () => {
    const wrapper = mount(VirtualDataViewport, {
      props: {
        items: Array.from({ length: 30 }, (_, index) => ({ id: `row-${index}`, label: `Row ${index}` })),
        itemHeight: fallbackRowHeight,
        viewportHeight,
        getItemKey: (item: { id: string }) => item.id,
      },
      slots: {
        default: ({ item }: { item: { label: string } }) => item.label,
      },
    })

    await flushPromises()
    await wrapper.vm.$nextTick()

    const scroller = wrapper.get('.data-viewport__scroller').element as HTMLElement
    let internalScrollTop = 0
    Object.defineProperty(scroller, 'clientHeight', {
      configurable: true,
      value: viewportHeight,
    })
    Object.defineProperty(scroller, 'scrollTop', {
      configurable: true,
      get: () => internalScrollTop,
      set: (value: number) => {
        internalScrollTop = Math.floor(value)
      },
    })
    Object.defineProperty(scroller, 'scrollHeight', {
      configurable: true,
      get: () => (wrapper.props('items') as Array<unknown>).length * fallbackRowHeight,
    })

    scroller.scrollTop = 1400
    await wrapper.get('.data-viewport__scroller').trigger('scroll')
    await wrapper.vm.$nextTick()

    await wrapper.setProps({
      items: Array.from({ length: 4 }, (_, index) => ({ id: `row-${index}`, label: `Row ${index}` })),
    })
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(internalScrollTop).toBe(Math.max(0, (4 * fallbackRowHeight) - viewportHeight))
    expect(wrapper.findAll('.data-viewport__row')).not.toHaveLength(0)
  })
})
