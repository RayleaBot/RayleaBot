import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AuthParticleField from '@/components/auth/AuthParticleField.vue'

const palette = {
  line: 'rgb(11 107 143 / 17%)',
  particle: 'rgb(11 107 143 / 66%)',
}

function createMediaQueryList(media: string, matches: boolean): MediaQueryList {
  return {
    matches,
    media,
    onchange: null,
    addEventListener: vi.fn(),
    addListener: vi.fn(),
    dispatchEvent: vi.fn(),
    removeEventListener: vi.fn(),
    removeListener: vi.fn(),
  }
}

function installMediaQueries(options: { finePointer: boolean, reducedMotion: boolean }) {
  window.matchMedia = vi.fn((query: string) => createMediaQueryList(
    query,
    query.includes('(any-hover: hover)') ? options.finePointer : options.reducedMotion,
  ))
}

function installCanvasContext() {
  const context = {
    arc: vi.fn(),
    beginPath: vi.fn(),
    clearRect: vi.fn(),
    fill: vi.fn(),
    fillStyle: '',
    globalAlpha: 1,
    lineTo: vi.fn(),
    lineWidth: 1,
    moveTo: vi.fn(),
    setTransform: vi.fn(),
    stroke: vi.fn(),
    strokeStyle: '',
  }
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(
    context as unknown as CanvasRenderingContext2D,
  )
  return context
}

describe('AuthParticleField', () => {
  let documentHidden = false

  beforeEach(() => {
    documentHidden = false
    vi.spyOn(document, 'hidden', 'get').mockImplementation(() => documentHidden)
    installCanvasContext()
  })

  it('runs one animation loop, tracks the pointer, and cancels the frame on unmount', () => {
    installMediaQueries({ finePointer: true, reducedMotion: false })
    const context = installCanvasContext()
    const callbacks = new Map<number, FrameRequestCallback>()
    const requestFrame = vi.spyOn(window, 'requestAnimationFrame').mockImplementation((callback) => {
      const frameId = callbacks.size + 1
      callbacks.set(frameId, callback)
      return frameId
    })
    const cancelFrame = vi.spyOn(window, 'cancelAnimationFrame').mockImplementation(() => {})
    const wrapper = mount(AuthParticleField, { props: { palette } })
    const canvas = wrapper.get('canvas')

    expect(canvas.attributes('data-auth-particle-state')).toBe('running')
    expect(requestFrame).toHaveBeenCalledTimes(1)
    window.dispatchEvent(new PointerEvent('pointermove', { clientX: 120, clientY: 160 }))
    expect(canvas.attributes('data-auth-pointer-active')).toBe('true')

    const drawsAfterMount = context.clearRect.mock.calls.length
    callbacks.get(1)?.(8.3)
    callbacks.get(2)?.(16.6)
    expect(requestFrame).toHaveBeenCalledTimes(3)
    expect(context.clearRect).toHaveBeenCalledTimes(drawsAfterMount + 2)
    wrapper.unmount()
    expect(cancelFrame).toHaveBeenCalledWith(3)
  })

  it('renders a static field and skips the animation loop for reduced motion', () => {
    installMediaQueries({ finePointer: true, reducedMotion: true })
    const requestFrame = vi.spyOn(window, 'requestAnimationFrame')
    const context = installCanvasContext()
    const wrapper = mount(AuthParticleField, { props: { palette } })

    expect(wrapper.get('canvas').attributes('data-auth-particle-state')).toBe('static')
    expect(requestFrame).not.toHaveBeenCalled()
    expect(context.fill).toHaveBeenCalled()
    wrapper.unmount()
  })

  it('pauses the loop while the page is hidden', () => {
    installMediaQueries({ finePointer: false, reducedMotion: false })
    vi.spyOn(window, 'requestAnimationFrame').mockImplementation(() => 7)
    const cancelFrame = vi.spyOn(window, 'cancelAnimationFrame').mockImplementation(() => {})
    const wrapper = mount(AuthParticleField, { props: { palette } })

    documentHidden = true
    document.dispatchEvent(new Event('visibilitychange'))
    expect(wrapper.get('canvas').attributes('data-auth-particle-state')).toBe('paused')
    expect(cancelFrame).toHaveBeenCalledWith(7)
    wrapper.unmount()
  })
})
