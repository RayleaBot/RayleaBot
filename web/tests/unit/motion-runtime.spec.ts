import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Router } from 'vue-router'

const animateMock = vi.hoisted(() => vi.fn())

vi.mock('motion/mini', () => ({ animate: animateMock }))

import {
  navigateWithMotion,
  runRouteFallbackMotion,
} from '@/motion/runtime'

function installViewTransitionMock() {
  const transitions: Array<{ resolve: () => void; skipTransition: ReturnType<typeof vi.fn> }> = []
  const startViewTransition = vi.fn((update: () => void | Promise<void>) => {
    let resolve = () => {}
    const finished = new Promise<void>((next) => { resolve = next })
    const skipTransition = vi.fn()
    const updateCallbackDone = Promise.resolve(update()).then(() => undefined)
    transitions.push({ resolve, skipTransition })
    return {
      finished,
      ready: Promise.resolve(),
      updateCallbackDone,
      skipTransition,
    } as ViewTransition
  })
  Object.defineProperty(document, 'startViewTransition', {
    configurable: true,
    value: startViewTransition,
  })
  return { startViewTransition, transitions }
}

function createRouterMock() {
  return {
    push: vi.fn(async () => undefined),
  } as unknown as Router
}

describe('motion runtime', () => {
  beforeEach(() => {
    animateMock.mockReset()
    Reflect.deleteProperty(document, 'startViewTransition')
    delete document.documentElement.dataset.viewTransitionKind
    delete document.documentElement.dataset.motionProfile
    vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: false })))
  })

  it('falls back to direct navigation when view transitions are unavailable', async () => {
    const router = createRouterMock()

    await navigateWithMotion(router, '/logs', 'fade')

    expect(router.push).toHaveBeenCalledOnce()
    expect(router.push).toHaveBeenCalledWith('/logs')
  })

  it('uses a named view transition for animated navigation', async () => {
    const router = createRouterMock()
    const { startViewTransition, transitions } = installViewTransitionMock()

    const navigation = navigateWithMotion(router, '/plugins', 'fade-slide')

    expect(startViewTransition).toHaveBeenCalledOnce()
    expect(document.documentElement.dataset.viewTransitionKind).toBe('route')
    expect(document.documentElement.dataset.motionProfile).toBe('fade-slide')
    await navigation
    transitions[0]?.resolve()
  })

  it('skips an active view transition before starting the latest navigation', () => {
    const router = createRouterMock()
    const { transitions } = installViewTransitionMock()

    void navigateWithMotion(router, '/commands', 'fade')
    void navigateWithMotion(router, '/logs', 'fade')

    expect(transitions[0]?.skipTransition).toHaveBeenCalledOnce()
    transitions.forEach((transition) => transition.resolve())
  })

  it('uses Motion Mini for the route fallback and completes through its controls', async () => {
    const element = document.createElement('div')
    Object.defineProperty(element, 'animate', { configurable: true, value: vi.fn() })
    const done = vi.fn()
    const controls = Object.assign(Promise.resolve(), { cancel: vi.fn() })
    animateMock.mockReturnValue(controls)

    runRouteFallbackMotion(element, 'enter', 'fade-slide', done)
    await controls

    expect(animateMock).toHaveBeenCalledWith(
      element,
      expect.objectContaining({ opacity: [0, 1] }),
      expect.objectContaining({ duration: 0.2 }),
    )
    expect(done).toHaveBeenCalledOnce()
  })

  it('completes the fallback leave phase immediately so one navigation lasts 200ms', async () => {
    const element = document.createElement('div')
    const done = vi.fn()

    runRouteFallbackMotion(element, 'leave', 'fade-slide', done)
    await Promise.resolve()

    expect(animateMock).not.toHaveBeenCalled()
    expect(done).toHaveBeenCalledOnce()
  })

  it('disables all optional motion for reduced-motion users', async () => {
    vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: true })))
    const router = createRouterMock()
    const { startViewTransition } = installViewTransitionMock()

    await navigateWithMotion(router, '/logs', 'fade-slide')

    expect(startViewTransition).not.toHaveBeenCalled()
    expect(router.push).toHaveBeenCalledOnce()
  })
})
