import { afterEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { lazyRouteComponents, prefetchRouteComponents } from '@/router/prefetch'

const Page = { template: '<div />' }

function createFixtureRouter(loaders: Array<() => Promise<unknown>>) {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: Page },
      { path: '/a', component: loaders[0] as never },
      { path: '/b', component: loaders[1] as never },
      { path: '/__dev/components', component: loaders[2] as never },
    ],
  })
}

afterEach(() => {
  vi.useRealTimers()
})

describe('route prefetch', () => {
  it('collects lazy page loaders and skips development pages', () => {
    const loaders = [vi.fn(), vi.fn(), vi.fn()].map(fn => fn.mockResolvedValue(Page))
    expect(lazyRouteComponents(createFixtureRouter(loaders))).toEqual([loaders[0], loaders[1]])
  })

  it('loads one page per idle turn after the start delay', async () => {
    vi.useFakeTimers()
    const loaders = [vi.fn(), vi.fn(), vi.fn()].map(fn => fn.mockResolvedValue(Page))
    const stop = prefetchRouteComponents(createFixtureRouter(loaders))

    await vi.advanceTimersByTimeAsync(1500)
    expect(loaders[0]).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(200)
    expect(loaders[0]).toHaveBeenCalledTimes(1)
    expect(loaders[1]).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(200)
    expect(loaders[1]).toHaveBeenCalledTimes(1)
    stop()
  })

  it('stops prefetching when cancelled', async () => {
    vi.useFakeTimers()
    const loaders = [vi.fn(), vi.fn(), vi.fn()].map(fn => fn.mockResolvedValue(Page))
    const stop = prefetchRouteComponents(createFixtureRouter(loaders))

    await vi.advanceTimersByTimeAsync(1700)
    expect(loaders[0]).toHaveBeenCalledTimes(1)
    stop()
    await vi.advanceTimersByTimeAsync(10000)
    expect(loaders[1]).not.toHaveBeenCalled()
    expect(loaders[2]).not.toHaveBeenCalled()
  })
})
