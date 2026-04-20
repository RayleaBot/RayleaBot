import { config } from '@vue/test-utils'
import { afterEach, beforeEach, vi } from 'vitest'

class ResizeObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
}

beforeEach(() => {
  Object.defineProperty(window, 'ResizeObserver', {
    configurable: true,
    writable: true,
    value: ResizeObserverMock,
  })

  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  })

  const nativeGetComputedStyle = globalThis.getComputedStyle.bind(globalThis)
  Object.defineProperty(window, 'getComputedStyle', {
    configurable: true,
    writable: true,
    value: ((element: Element) => nativeGetComputedStyle(element)) as typeof window.getComputedStyle,
  })
})

afterEach(() => {
  window.sessionStorage.clear()
  vi.restoreAllMocks()
})
