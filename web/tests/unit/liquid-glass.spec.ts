import { afterEach, describe, expect, it, vi } from 'vitest'

import { GlassFilterRegistry, displacementPixels, lensGeometry, rendersInSoftware, rimShift } from '@/lib/liquid-glass'

afterEach(() => {
  vi.useRealTimers()
})

describe('software rendering probe', () => {
  function stubWebGL(renderer: string | null) {
    const context = renderer === null ? null : {
      RENDERER: 0x1f01,
      getExtension: (name: string) => (name === 'WEBGL_debug_renderer_info' ? { UNMASKED_RENDERER_WEBGL: 0x9246 } : null),
      getParameter: () => renderer,
    }
    return vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(context as never)
  }

  it('treats SwiftShader and other CPU renderers as software', () => {
    stubWebGL('ANGLE (Google, Vulkan 1.3.0 (SwiftShader Device (Subzero) (0x0000C0DE)), SwiftShader driver)')
    expect(rendersInSoftware()).toBe(true)
    stubWebGL('llvmpipe (LLVM 17.0.6, 256 bits)')
    expect(rendersInSoftware()).toBe(true)
  })

  it('keeps hardware renderers and treats a refused context as software', () => {
    stubWebGL('ANGLE (NVIDIA, NVIDIA GeForce RTX 3070 Laptop GPU Direct3D11 vs_5_0 ps_5_0, D3D11)')
    expect(rendersInSoftware()).toBe(false)
    stubWebGL(null)
    expect(rendersInSoftware()).toBe(true)
  })
})

describe('shell glass lenses', () => {
  it('bends light only inside the bevel and most strongly at the rim', () => {
    const geometry = lensGeometry({ width: 244, height: 876, radius: 28 })
    expect(rimShift(1, geometry)).toBe(0)
    expect(rimShift(0.9, geometry)).toBeGreaterThan(0)
    expect(rimShift(0.2, geometry)).toBeGreaterThan(rimShift(0.6, geometry))
    // Small controls keep a usable bevel; large panes stop growing it.
    expect(lensGeometry({ width: 12, height: 12, radius: 6 }).bevel).toBe(4)
    expect(lensGeometry({ width: 1200, height: 900, radius: 24 }).bevel).toBe(36)
  })

  it('renders tall lens maps at a bounded size with the lens aspect ratio', () => {
    const spec = { width: 244, height: 1056, radius: 28 }
    const map = displacementPixels(spec, lensGeometry(spec))
    expect(Math.max(map.width, map.height)).toBe(512)
    expect(map.width / map.height).toBeCloseTo(244 / 1056, 2)
    expect(map.pixels.length).toBe(map.width * map.height * 4)
    // The center stays clear; only the bevel bends the backdrop.
    const center = (Math.floor(map.height / 2) * map.width + Math.floor(map.width / 2)) * 4
    expect([map.pixels[center], map.pixels[center + 1]]).toEqual([128, 128])
  })

  it('removes filters for sizes no longer on screen after the release delay', () => {
    vi.useFakeTimers()
    const destroyed: string[] = []
    const registry = new GlassFilterRegistry<string>(
      (id) => ({ resource: id, ready: Promise.resolve(true) }),
      (resource) => destroyed.push(resource),
      1000,
    )

    const sidebar = registry.acquire('sidebar-720')
    registry.acquire('sidebar-720')
    registry.release('sidebar-720')
    vi.advanceTimersByTime(1000)
    expect(destroyed).toEqual([])

    registry.release('sidebar-720')
    const resized = registry.acquire('sidebar-640')
    registry.release('sidebar-640')
    vi.advanceTimersByTime(999)
    expect(registry.size).toBe(2)

    // A lens that returns to a size before the delay reuses the same filter.
    expect(registry.acquire('sidebar-720').id).toBe(sidebar.id)
    registry.release('sidebar-720')
    vi.advanceTimersByTime(1000)

    expect(destroyed).toEqual([resized.id, sidebar.id])
    expect(registry.size).toBe(0)
  })
})
